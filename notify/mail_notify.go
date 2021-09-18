package notify

// Inspired from https://github.com/zbindenren/logrus_mail
import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"os"
	"strconv"
	"strings"
	"time"
)

type MailNotify struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Host     string `json:"smtpHost"`
	Port     int    `json:"port"`
	From     string `json:"from"`
	To       string `json:"to"`
	Cc       string `json:"cc"`
}

var (
	MaxLineLength = 76 // MaxLineLength is the maximum line length per RFC 2045
	isAuthorized  bool
	client        *smtp.Client
	maxBigInt     = big.NewInt(math.MaxInt64)
	// TLS config
	tlsconfig *tls.Config = &tls.Config{
		InsecureSkipVerify: true,
	}
)

const smtpTimeout = 3 * time.Second

func (mailNotify MailNotify) GetClientName() string {
	return "Smtp Mail"
}

func (mailNotify MailNotify) connect() error {
	conn, err := net.DialTimeout("tcp", mailNotify.Host+":"+strconv.Itoa(mailNotify.Port), smtpTimeout)
	if err != nil {
		return err
	}
	if mailNotify.Port == 465 {
		tlsConn := tls.Client(conn, tlsconfig)
		err = tlsConn.Handshake()
		if err != nil {
			return err
		}
		conn = tlsConn
	}
	client, err = smtp.NewClient(conn, mailNotify.Host)
	if err != nil {
		return err
	}

	// Check if server supports starttls
	if ok, _ := client.Extension("STARTTLS"); ok {
		err = client.StartTLS(tlsconfig)
	}
	if err != nil {
		return err
	}

	if isAuthorized {
		smtpAuth := smtp.PlainAuth("", mailNotify.Username, mailNotify.Password, mailNotify.Host)
		if ok, _ := client.Extension("AUTH"); ok {
			if err = client.Auth(smtpAuth); err != nil {
				return fmt.Errorf("Error while Auth with SMTP Server: %s, error: %v", mailNotify.Host, err)
			}
		}
	}

	return nil
}

func (mailNotify MailNotify) Initialize() error {
	var err error

	tlsconfig.ServerName = mailNotify.Host

	// Validate sender and recipient
	_, err = mail.ParseAddress(mailNotify.From)
	if err != nil {
		return err
	}
	_, err = mail.ParseAddress(mailNotify.To)
	if err != nil {
		return err
	}

	if len(mailNotify.From) < 1 || len(mailNotify.To) < 1 {
		return errors.New("Must specify at least one From address and one To address")
	}

	// TODO: validate port and email host
	if len(mailNotify.Username) == 0 && len(mailNotify.Password) == 0 {
		isAuthorized = false
	} else {
		isAuthorized = true
	}

	// Check server connection.
	err = mailNotify.connect()
	if err != nil {
		return err
	}
	return client.Close()
}

func (mailNotify MailNotify) sendEmail(subject string, message string) error {
	var err error

	if err = mailNotify.connect(); err != nil {
		return err
	}

	if err = client.Mail(mailNotify.From); err != nil {
		return err
	}
	if err = client.Rcpt(mailNotify.To); err != nil {
		return err
	}

	raw, err := mailNotify.Bytes(subject, message)
	if err != nil {
		return err
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	_, err = w.Write(raw)
	if err != nil {
		return err
	}
	err = w.Close()
	if err != nil {
		return err
	}
	return client.Quit()
}

// msgHeaders merges the Email's various fields and custom headers together in a
// standards compliant way to create a MIMEHeader to be used in the resulting
// message. It does not alter e.Headers.
//
// "e"'s fields To, Cc, From, Subject will be used unless they are present in
// e.Headers. Unless set in e.Headers, "Date" will filled with the current time.
func (e MailNotify) msgHeaders(subject string) (textproto.MIMEHeader, error) {
	// res := make(textproto.MIMEHeader, len(e.Headers)+6)
	res := make(textproto.MIMEHeader, 6)

	// Set headers if there are values.
	/*
		if _, ok := res["Reply-To"]; !ok && len(e.ReplyTo) > 0 {
			res.Set("Reply-To", strings.Join(e.ReplyTo, ", "))
		}
	*/
	if _, ok := res["To"]; !ok && len(e.To) > 0 {
		// res.Set("To", strings.Join(e.To, ", "))
		res.Set("To", e.To)
	}

	if _, ok := res["Cc"]; !ok && len(e.Cc) > 0 {
		// res.Set("Cc", strings.Join(e.Cc, ", "))
		res.Set("Cc", e.Cc)
	}

	if _, ok := res["Subject"]; !ok && subject != "" {
		res.Set("Subject", subject)
	}
	if _, ok := res["Message-Id"]; !ok {
		id, err := generateMessageID()
		if err != nil {
			return nil, err
		}
		res.Set("Message-Id", id)
	}
	// Date and From are required headers.
	if _, ok := res["From"]; !ok {
		res.Set("From", e.From)
	}
	if _, ok := res["Date"]; !ok {
		res.Set("Date", time.Now().Format(time.RFC1123Z))
	}
	if _, ok := res["MIME-Version"]; !ok {
		res.Set("MIME-Version", "1.0")
	}

	return res, nil
}

// Bytes converts the Email object to a []byte representation, including all needed MIMEHeaders, boundaries, etc.
func (e MailNotify) Bytes(subject string, message string) ([]byte, error) {
	// TODO: better guess buffer size
	buff := bytes.NewBuffer(make([]byte, 0, 4096))

	headers, err := e.msgHeaders(subject)
	if err != nil {
		return nil, err
	}
	var w *multipart.Writer

	headers.Set("Content-Type", "text/plain; charset=UTF-8")
	headers.Set("Content-Transfer-Encoding", "quoted-printable")

	headerToBytes(buff, headers)
	_, err = io.WriteString(buff, "\r\n")
	if err != nil {
		return nil, err
	}

	// Check to see if there is a Text or HTML field
	// if len(e.Text) > 0 || len(e.HTML) > 0 {
	if len(message) > 0 {
		// Create the body sections
		// Write the text
		if err := writeMessage(buff, []byte(message), false, "text/plain", w); err != nil {
			return nil, err
		}
	}

	return buff.Bytes(), nil
}

func (mailNotify MailNotify) SendResponseTimeNotification(responseTimeNotification ResponseTimeNotification) error {
	subject := "Monitoring Notification: Response Time"
	message := getMessageFromResponseTimeNotification(responseTimeNotification)

	return mailNotify.sendEmail(subject, message)
}

func (mailNotify MailNotify) SendErrorNotification(errorNotification ErrorNotification) error {
	subject := "Monitoring Alert! Error"
	message := getMessageFromErrorNotification(errorNotification)

	return mailNotify.sendEmail(subject, message)
}

func writeMessage(buff io.Writer, msg []byte, multipart bool, mediaType string, w *multipart.Writer) error {
	if multipart {
		header := textproto.MIMEHeader{
			"Content-Type":              {mediaType + "; charset=UTF-8"},
			"Content-Transfer-Encoding": {"quoted-printable"},
		}
		if _, err := w.CreatePart(header); err != nil {
			return err
		}
	}

	qp := quotedprintable.NewWriter(buff)
	// Write the text
	if _, err := qp.Write(msg); err != nil {
		return err
	}
	return qp.Close()
}

// headerToBytes renders "header" to "buff". If there are multiple values for a
// field, multiple "Field: value\r\n" lines will be emitted.
func headerToBytes(buff io.Writer, header textproto.MIMEHeader) {
	for field, vals := range header {
		for _, subval := range vals {
			// bytes.Buffer.Write() never returns an error.
			io.WriteString(buff, field)
			io.WriteString(buff, ": ")
			// Write the encoded header if needed
			switch {
			case field == "Content-Type" || field == "Content-Disposition":
				buff.Write([]byte(subval))
			case field == "From" || field == "To" || field == "Cc" || field == "Bcc":
				participants := strings.Split(subval, ",")
				for i, v := range participants {
					addr, err := mail.ParseAddress(v)
					if err != nil {
						continue
					}
					participants[i] = addr.String()
				}
				buff.Write([]byte(strings.Join(participants, ", ")))
			default:
				buff.Write([]byte(mime.QEncoding.Encode("UTF-8", subval)))
			}
			io.WriteString(buff, "\r\n")
		}
	}
}

// generateMessageID generates and returns a string suitable for an RFC 2822
// compliant Message-ID, e.g.:
// <1444789264909237300.3464.1819418242800517193@DESKTOP01>
//
// The following parameters are used to generate a Message-ID:
// - The nanoseconds since Epoch
// - The calling PID
// - A cryptographically random int64
// - The sending hostname
func generateMessageID() (string, error) {
	t := time.Now().UnixNano()
	pid := os.Getpid()
	rint, err := rand.Int(rand.Reader, maxBigInt)
	if err != nil {
		return "", err
	}
	h, err := os.Hostname()
	// If we can't get the hostname, we'll use localhost
	if err != nil {
		h = "localhost.localdomain"
	}
	msgid := fmt.Sprintf("<%d.%d.%d@%s>", t, pid, rint, h)
	return msgid, nil
}
