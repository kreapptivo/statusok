package database

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"statusok/model"
	"strconv"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
)

type InfluxDb struct {
	Host   string `json:"host"`
	Port   int    `json:"port"`
	Bucket string `json:"bucket"`
	Org    string `json:"org"`
	Token  string `json:"token"`
}

var influxDBcon influxdb2.Client

const (
	DatabaseName = "InfluxDB"
)

// Return database name
func (influxDb *InfluxDb) GetDatabaseName() string {
	return DatabaseName
}

// Check if necessary data is given
func (influxDb *InfluxDb) IsEmpty() bool {
	if influxDb.Host == "" ||
		influxDb.Port == 0 || influxDb.Bucket == "" || influxDb.Org == "" || influxDb.Token == "" {
		return true
	}
	return false
}

// Intiliaze influx db
func (influxDb *InfluxDb) Initialize() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// TODO: check config variables!

	fmt.Printf("InfluxDB : Trying to Connect to host %s\n", influxDb.Host)

	databaseUri := fmt.Sprintf("http://%s:%d", influxDb.Host, influxDb.Port)
	_, err := url.Parse(databaseUri)
	if err != nil {
		return fmt.Errorf("InfluxDB : Invalid Url \"%s\". Please check domain name given in config file!\nError Details: %s", databaseUri, err.Error())
	}

	influxDBcon = influxdb2.NewClient(databaseUri, influxDb.Token)

	check, err := influxDBcon.Health(ctx)
	if err != nil {
		return fmt.Errorf("InfluxDB : Failed to connect to Database %s with Token %s. Please check the details entered in the config file!\nError Details: %s\n", databaseUri, influxDb.Token, err.Error())
	}

	if check.Status == "pass" {
		fmt.Printf("InfluxDB: Successfuly connected to InfluxDB version: %s\n", *check.Version)
		return nil
	}

	return fmt.Errorf("InfluxDB: Database not ready, got %s for state: %s\n", check.Status, *check.Message)
}

// Add request information to database
func (influxDb *InfluxDb) AddRequestInfo(requestInfo model.RequestInfo) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tags := map[string]string{
		"requestId":   strconv.Itoa(requestInfo.Id),
		"requestType": requestInfo.RequestType,
	}
	fields := map[string]interface{}{
		"responseTimeMs": requestInfo.ResponseTimeMs,
		"responseCode":   requestInfo.ResponseCode,
	}

	writeAPI := influxDBcon.WriteAPIBlocking(influxDb.Org, influxDb.Bucket)

	// Create point using full params constructor
	p := influxdb2.NewPoint(requestInfo.Url, tags, fields, time.Now())

	// Write point immediately
	err := writeAPI.WritePoint(ctx, p)
	if err != nil {
		fmt.Printf("Influxdb: could not write entry to db, error: %s\n", err)
		return err
	}
	// DEBUG: fmt.Printf("Influxb: wrote new point %+v\n", p)
	return nil
}

// Add Error information to database
func (influxDb *InfluxDb) AddErrorInfo(errorInfo model.ErrorInfo) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tags := map[string]string{
		"requestId":   strconv.Itoa(errorInfo.Id),
		"requestType": errorInfo.RequestType,
		"reason":      errorInfo.Reason.Error(),
	}
	fields := map[string]interface{}{
		"responseBody": errorInfo.ResponseBody,
		"responseCode": errorInfo.ResponseCode,
		"otherInfo":    errorInfo.OtherInfo,
	}

	writeAPI := influxDBcon.WriteAPIBlocking(influxDb.Org, influxDb.Bucket)

	// Create point using full params constructor
	p := influxdb2.NewPoint(errorInfo.Url,
		tags,
		fields,
		time.Now(),
	)

	// Write point immediately
	err := writeAPI.WritePoint(ctx, p)
	if err != nil {
		return err
	}

	return nil
}

// Returns mean response time of url in given time .Currentlt not used
func (influxDb *InfluxDb) GetMeanResponseTime(Url string, span int) (float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	q := fmt.Sprintf(`select mean(responseTime) from "%s" WHERE time > now() - %dm GROUP BY time(%dm)`, Url, span, span)

	// Get query client
	queryAPI := influxDBcon.QueryAPI(influxDb.Org)

	//`from(bucket:"my-bucket")|> range(start: -1h) |> filter(fn: (r) => r._measurement == "stat")`)

	// Get parser flux query result
	result, err := queryAPI.Query(ctx, q)
	if err != nil {
		fmt.Printf("Got error %s for query %s", err, q)
		return 0, err
	}

	// Use Next() to iterate over query result lines
	for result.Next() {
		// Observe when there is new grouping key producing new table
		if result.TableChanged() {
			fmt.Printf("table: %s\n", result.TableMetadata().String())
		}
		// read result
		fmt.Printf("row: %s\n", result.Record().String())
	}

	return 0, errors.New("not yet fully implemented")
}
