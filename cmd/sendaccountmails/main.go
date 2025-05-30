// This file is Free Software under the Apache-2.0 License
// without warranty, see README.md and LICENSE for details.
//
// SPDX-License-Identifier: Apache-2.0
//
// SPDX-FileCopyrightText: 2025 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
// Software-Engineering: 2025 Intevation GmbH <https://intevation.de>

// Package main implements sending out emails for new accounts
// as being read from an CSV file in the format `emailaddress,password`.
// where `emailaddress` is the username by convention.
package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log"
	"net/smtp"
	"os"
	"strings"
	"text/template"
)

const templateTxt = `Dear OASIS {{.TCName}} TC member,

an account was created for you at the OQC (https://quorum.oasis-open.org).

username: {{.Recipient}}
initial password: {{.Password}}

Please change your initial password.

Kind regards,
Your OQC Tool`

func check(err error) {
	if err != nil {
		log.Fatalf("error: %v\n", err)
	}
}

func send(host, sender, recipient string,
	writeBody func(io.Writer) error,
) error {
	c, err := smtp.Dial(host)
	if err != nil {
		return err
	}
	defer c.Close()

	// Set the sender and recipient first
	if err := c.Mail(sender); err != nil {
		return err
	}
	if err := c.Rcpt(recipient); err != nil {
		return err
	}

	// Send the email body.
	wc, err := c.Data()
	if err != nil {
		return err
	}
	if err := writeBody(wc); err != nil {
		return err
	}
	if err = wc.Close(); err != nil {
		return err
	}

	// Send the QUIT command and close the connection.
	if err = c.Quit(); err != nil {
		return err
	}
	return nil
}

func sendMail(
	tmpl *template.Template,
	recipient, password, TCName, smtpHost string) error {
	smtpPort := "25"
	emailFrom := "OASIS Quorum Calculator <no-reply@quorum.oasis-open.org>"
	//emailPassword := ""

	subject := "OQC - OASIS Quorum Calculator: Account creation"

	data := struct {
		Recipient string
		Password  string
		TCName    string
	}{
		Recipient: recipient,
		Password:  password,
		TCName:    TCName,
	}

	writeBody := func(body io.Writer) error {
		fmt.Fprintf(body, "To: %s\r\n", recipient)
		fmt.Fprintf(body, "From: %s\r\n", emailFrom)
		fmt.Fprintf(body, "Subject: %s\r\n", subject)
		fmt.Fprint(body, "MIME-Version: 1.0\r\n")
		fmt.Fprint(body, "Content-Transfer-Encoding: 8bit\r\n")
		fmt.Fprint(body, "Content-Type: text/plain; charset=\"UTF-8\"\r\n")
		fmt.Fprint(body, "\r\n")
		if err := tmpl.Execute(body, data); err != nil {
			return err
		}
		_, err := fmt.Fprint(body, "\r\n")
		return err
	}

	//auth := smtp.PlainAuth("", emailFrom, emailPassword, smtpHost)

	if err := send(
		smtpHost+":"+smtpPort, emailFrom, recipient, writeBody); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}
	log.Printf("Email to %s sent successfully!\n", recipient)

	return nil
}

func run(tmplText, passwordCSV, TCName, smtpHost string) error {
	passwordsFile, err := os.Open(passwordCSV)
	if err != nil {
		return err
	}
	defer passwordsFile.Close()

	r := csv.NewReader(passwordsFile)
	records, err := r.ReadAll()
	if err != nil {
		return err
	}

	// make sure that mixed line endings are all \r\n
	tmplText = strings.ReplaceAll(tmplText, "\r\n", "\n")
	tmplText = strings.ReplaceAll(tmplText, "\n", "\r\n")

	tmpl, err := template.New("body").Parse(tmplText)
	if err != nil {
		return err
	}

	log.Printf("sending out emails for TC `%s`\n", TCName)
	for _, record := range records {
		if err := sendMail(tmpl, record[0], record[1], TCName, smtpHost); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	var (
		passwordCSV string
		TCName      string
		smtpHost    string
	)

	flag.StringVar(&passwordCSV, "p", "passwords.csv", "CSV file of the list of users and passwords.")

	flag.StringVar(&TCName, "t", "", "Name of the TC to mention in the email.")
	flag.StringVar(&smtpHost, "h", "localhost", "Name of the smtp server to connect to.")
	flag.Parse()

	check(run(templateTxt, passwordCSV, TCName, smtpHost))
}
