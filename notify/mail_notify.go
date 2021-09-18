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
	// Check if server listens on that port.

	// TLS config
	tlsconfig := &tls.Config{
		InsecureSkipVerify: false,
		ServerName:         mailNotify.Host,
	}

	var err error

	if mailNotify.Port == 465 {
		conn, err := tls.Dial("tcp", mailNotify.Host+":"+strconv.Itoa(mailNotify.Port), tlsconfig)
		if err != nil {
			return err
		}
		client, err = smtp.NewClient(conn, mailNotify.Host+":"+strconv.Itoa(mailNotify.Port))
	} else if mailNotify.Port == 587 {
		// conn, err = net.DialTimeout("tcp", mailNotify.Host+":"+strconv.Itoa(mailNotify.Port), 3*time.Second)
		conn, err := smtp.Dial(mailNotify.Host + ":" + strconv.Itoa(mailNotify.Port))
		if err != nil {
			return err
		}
		conn.StartTLS(tlsconfig)
		client = conn
	} else {
		// conn, err := smtp.Dial(mailNotify.Host + ":" + strconv.Itoa(mailNotify.Port))
		conn, err := net.DialTimeout("tcp", mailNotify.Host+":"+strconv.Itoa(mailNotify.Port), 3*time.Second)
		if err != nil {
			return err
		}
		client, err = smtp.NewClient(conn, mailNotify.Host+":"+strconv.Itoa(mailNotify.Port))
	}
	if err != nil {
		return err
	}

	if len(mailNotify.Username) == 0 && len(mailNotify.Password) == 0 {
		isAuthorized = false
	} else {
		isAuthorized = true

		// auth := smtp.PlainAuth("",mailNotify.Username, mailNotify.Password, mailNotifyHost)

		// Auth
		//if err = conn.Auth(auth); err != nil {
		//    return err
		//}
	}
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

	return nil
}

func (mailNotify MailNotify) SendResponseTimeNotification(responseTimeNotification ResponseTimeNotification) error {
	if isAuthorized {

		auth := smtp.PlainAuth("", mailNotify.Username, mailNotify.Password, mailNotify.Host)

		message := getMessageFromResponseTimeNotification(responseTimeNotification)

		// Connect to the server, authenticate, set the sender and recipient,
		// and send the email all in one step.
		err := smtp.SendMail(
			mailNotify.Host+":"+strconv.Itoa(mailNotify.Port),
			auth,
			mailNotify.From,
			[]string{mailNotify.To},
			bytes.NewBufferString(message).Bytes(),
		)
		if err != nil {
			return err
		}
		return nil
	} else {
		wc, err := client.Data()
		if err != nil {
			return err
		}

		defer wc.Close()

		message := bytes.NewBufferString(getMessageFromResponseTimeNotification(responseTimeNotification))

		if _, err = message.WriteTo(wc); err != nil {
			return err
		}

		return nil
	}
}

func (mailNotify MailNotify) SendErrorNotification(errorNotification ErrorNotification) error {
	if isAuthorized {

		auth := smtp.PlainAuth("", mailNotify.Username, mailNotify.Password, mailNotify.Host)

		message := getMessageFromErrorNotification(errorNotification)

		// Connect to the server, authenticate, set the sender and recipient,
		// and send the email all in one step.
		err := smtp.SendMail(
			mailNotify.Host+":"+strconv.Itoa(mailNotify.Port),
			auth,
			mailNotify.From,
			[]string{mailNotify.To},
			bytes.NewBufferString(message).Bytes(),
		)
		if err != nil {
			return err
		}
		return nil
	} else {
		wc, err := client.Data()
		if err != nil {
			return err
		}

		defer wc.Close()

		message := bytes.NewBufferString(getMessageFromErrorNotification(errorNotification))

		if _, err = message.WriteTo(wc); err != nil {
			return err
		}

		return nil
	}
}
