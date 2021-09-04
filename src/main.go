package main

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/sirupsen/logrus"
	"gopkg.in/gomail.v2"
	"gopkg.in/ini.v1"
	"gopkg.in/natefinch/lumberjack.v2"
	"os"
	"strconv"
	"strings"
)

var sendFlag int = 0

func init() {
	customFormatter := new(logrus.TextFormatter)
	customFormatter.DisableQuote = true
	customFormatter.TimestampFormat = "2006-01-02 15:04:05.000"
	logrus.SetFormatter(customFormatter)
	logrus.SetOutput(&lumberjack.Logger{
		Filename: "idcheck.log",
		MaxSize:  500, //M
		MaxAge:   30,  //days
	})

}

func sendMail(body string) {
	cfgserver, err := ini.Load("config.ini")
	if err != nil {
		logrus.Error(err)
		os.Exit(1)
	}
	host := cfgserver.Section("mail").Key("host").Value()
	username := cfgserver.Section("mail").Key("username").Value()
	password := cfgserver.Section("mail").Key("password").Value()
	recipients := cfgserver.Section("mail").Key("recipients").Value()
	subject := cfgserver.Section("mail").Key("subject").Value()
	m := gomail.NewMessage()
	m.SetHeader("From", username)
	recvArr := strings.Split(recipients, ",")
	addresses := make([]string, len(recvArr))
	for i, recipient := range recvArr {
		addresses[i] = m.FormatAddress(recipient, "")
	}
	m.SetHeader("To", addresses...)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", body)
	d := gomail.NewDialer(host, 465, username, password)
	err2 := d.DialAndSend(m)
	if err2 != nil {
		logrus.Error(err2)
	}
	fmt.Println("邮件发送成功")
	logrus.Info("邮件发送成功")
}

func GetDB(url string) *sql.DB {
	db, err := sql.Open("mysql", url)
	if err != nil {
		panic(err)
	}
	return db
}

func GetSelectStmt(db *sql.DB) *sql.Stmt {
	sqlstr := "select " +
		"t1.TABLE_SCHEMA ," +
		"t1.TABLE_NAME ," +
		"t1.COLUMN_NAME ," +
		"t1.DATA_TYPE ," +
		"t1.COLUMN_TYPE ," +
		"t2.`AUTO_INCREMENT` ," +
		"CASE " +
		"t1.DATA_TYPE WHEN 'bigint' THEN if(locate('unsigned', t1.COLUMN_TYPE) = 0, round(t2.AUTO_INCREMENT / 9223372036854775807 * 100,2), round(t2.AUTO_INCREMENT / 18446744073709551615 * 100,2)) " +
		"WHEN 'int' THEN if(locate('unsigned', t1.COLUMN_TYPE) = 0, round(t2.AUTO_INCREMENT / 2147483647 * 100,2), round(t2.AUTO_INCREMENT / 4294967295 * 100,2)) " +
		"END 'percent' " +
		"from " +
		"information_schema.columns t1 " +
		"inner join information_schema.tables t2 on " +
		"t1.TABLE_SCHEMA = t2.TABLE_SCHEMA " +
		"and t1.TABLE_NAME = t2.TABLE_NAME " +
		"where " +
		"t1.TABLE_SCHEMA != 'mysql' " +
		"and t1.EXTRA = 'auto_increment' " +
		"and t1.DATA_TYPE in ('int', 'bigint') " +
		"order by " +
		"percent desc " +
		"limit 10"
	stmt, _ := db.Prepare(sqlstr)
	return stmt
}

func main() {

	cfgserver, err := ini.Load("config.ini")
	if err != nil {
		logrus.Error(err)
		os.Exit(1)
	}
	url := cfgserver.Section("server").Key("url").Value()
	threshold, _ := strconv.ParseFloat(cfgserver.Section("server").Key("threshold").Value(), 32)
	db := GetDB(url)
	stmtSelect := GetSelectStmt(db)
	var sb strings.Builder
	sb.WriteString("<table style=\"width:100%;\" cellpadding=\"2\" cellspacing=\"0\" border=\"1\" bordercolor=\"#000000\"><tbody>")
	sb.WriteString("<tr>")
	sb.WriteString("<td>" + "table_schema" + "</td>")
	sb.WriteString("<td>" + "table_name" + "</td>")
	sb.WriteString("<td>" + "column_name" + "</td>")
	sb.WriteString("<td>" + "data_type" + "</td>")
	sb.WriteString("<td>" + "column_type" + "</td>")
	sb.WriteString("<td>" + "auto_increment" + "</td>")
	sb.WriteString("<td>" + "percent" + "</td>")
	sb.WriteString("</tr>")

	rows, _ := stmtSelect.Query()
	for rows.Next() {
		var table_schema, table_name, column_name, data_type, column_type, auto_increment, percent string
		if err := rows.Scan(&table_schema, &table_name, &column_name, &data_type, &column_type, &auto_increment, &percent); err != nil {
			logrus.Error(err)
		}
		p, _ := strconv.ParseFloat(percent, 32)

		sb.WriteString("<tr>")
		sb.WriteString("<td>" + table_schema + "</td>")
		sb.WriteString("<td>" + table_name + "</td>")
		sb.WriteString("<td>" + column_name + "</td>")
		sb.WriteString("<td>" + data_type + "</td>")
		sb.WriteString("<td>" + column_type + "</td>")
		sb.WriteString("<td>" + auto_increment + "</td>")
		if p > threshold {
			sb.WriteString("<td><font color=\"red\">" + percent + "%" + "</font></td>")
		} else {
			sb.WriteString("<td>" + percent + "%" + "</td>")
		}
		sb.WriteString("</tr>")

		record := "table_schema:" + table_schema + ",table_name:" + table_name + ",column_name:" + column_name + ",data_type:" + data_type + ",auto_increment:" + auto_increment + ",percent:" + percent + "%"
		fmt.Println(record)
		logrus.Info(record)
		if p >= threshold {
			sendFlag = 1
		}

	}
	sb.WriteString("</tbody></table>")

	if sendFlag == 1 {
		sendMail(sb.String())
	}

}
