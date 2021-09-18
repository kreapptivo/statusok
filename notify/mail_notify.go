package notify

// Inspired from https://github.com/zbindenren/logrus_mail
import (
	"bytes"
	"crypto/tls"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"time"
)

type MailNotify struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Host     string `json:"smtpHost"`
	Port     int    `json:"port"`
	From     string `json:"from"`
	To       string `json:"to"`
}

var (
	isAuthorized bool
	client       *smtp.Client
)

func (mailNotify MailNotify) GetClientName() string {
	return "Smtp Mail"
}

func (mailNotify MailNotify) Initialize() error {
	var err error

	// Validate sender and recipient
	_, err = mail.ParseAddress(mailNotify.From)
	if err != nil {
		return err
	}
	_, err = mail.ParseAddress(mailNotify.To)
	// TODO: validate port and email host
	if err != nil {
		return err
	}

	// Check server connection.

	// TLS config
	tlsconfig := &tls.Config{
		InsecureSkipVerify: false,
		ServerName:         mailNotify.Host,
	}
	const timeout = 3 * time.Second

	conn, err := net.DialTimeout("tcp", mailNotify.Host+":"+strconv.Itoa(mailNotify.Port), timeout)
	if err != nil {
		return err
	}
	if mailNotify.Port == 465 {
		tlsConn := tls.Client(conn, tlsconfig)
		err = tlsConn.Handshake()
		if err != nil {
			return err
		}
		client, err = smtp.NewClient(tlsConn, mailNotify.Host+":"+strconv.Itoa(mailNotify.Port))
	} else if mailNotify.Port == 587 {
		client, err = smtp.NewClient(conn, mailNotify.Host+":"+strconv.Itoa(mailNotify.Port))
		client.StartTLS(tlsconfig)
	} else {
		client, err = smtp.NewClient(conn, mailNotify.Host+":"+strconv.Itoa(mailNotify.Port))
	}
	if err != nil {
		return err
	}

	if len(mailNotify.Username) == 0 && len(mailNotify.Password) == 0 {
		isAuthorized = false
	} else {
		isAuthorized = true
	}

	return nil
}

func (mailNotify MailNotify) sendEmail(message string) error {
	var smtpAuth smtp.Auth
	if isAuthorized {
		smtpAuth = smtp.PlainAuth("", mailNotify.Username, mailNotify.Password, mailNotify.Host)
	} else {
		smtpAuth = nil
	}
	err := smtp.SendMail(
		mailNotify.Host+":"+strconv.Itoa(mailNotify.Port),
		smtpAuth,
		mailNotify.From,
		[]string{mailNotify.To},
		bytes.NewBufferString(message).Bytes(),
	)
	if err != nil {
		return err
	}
	return nil
}

func (mailNotify MailNotify) SendResponseTimeNotification(responseTimeNotification ResponseTimeNotification) error {
	message := getMessageFromResponseTimeNotification(responseTimeNotification)

	return mailNotify.sendEmail(message)
}

func (mailNotify MailNotify) SendErrorNotification(errorNotification ErrorNotification) error {
	message := getMessageFromErrorNotification(errorNotification)

	return mailNotify.sendEmail(message)
}
