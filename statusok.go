package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"statusok/database"
	"statusok/logger"
	"statusok/notify"
	"statusok/requests"
	"time"

	"github.com/urfave/cli"
)

const (
	DefaultPort = 7321
)

type configuration struct {
	NotifyWhen    NotifyWhen               `json:"notifyWhen"`
	Requests      []requests.RequestConfig `json:"requests"`
	Notifications notify.NotificationTypes `json:"notifications"`
	Database      database.DatabaseTypes   `json:"database"`
	Concurrency   int                      `json:"concurrency"`
	Port          int                      `json:"port"`
}

type NotifyWhen struct {
	MinResponseCount int `json:"minResponseCount"`
	ErrorCount       int `json:"errorCount"`
}

func main() {
	// Cli tool setup to get config file path from parameters
	app := cli.NewApp()
	app.Name = "StatusOk"
	app.Usage = "Monitors websites. Get notified if a service is down."

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "config",
			Value: "config.json",
			Usage: "location of config file",
		},
		cli.StringFlag{
			Name:  "log",
			Value: "",
			Usage: "file to save logs",
		},
	}

	app.Action = func(c *cli.Context) {
		if len(c.String("config")) == 0 || !fileExists(c.String("config")) {
			fmt.Printf("Config file not present at the given location: %s\n. Please use correct file location with --config parameter", c.String("config"))
			return
		}

		if len(c.String("log")) != 0 && !logFilePathValid(c.String("log")) {
			// log parameter given.Check if file can be created at given path

			fmt.Printf("Invalid File Path given for parameter --log: %s", c.String("log"))
			os.Exit(3)
		}

		// parse config file
		fmt.Printf("Using config file : %s", c.String("config"))
		config, err := readConfig(c.String("config"))
		if err != nil {
			fmt.Println(err)
			os.Exit(3)
		}
		// Start monitoring when a valid file path is given
		startMonitoring(config, c.String("log"))
	}

	// Run as cli app
	err := app.Run(os.Args)
	if err != nil {
		fmt.Printf("Error starting Application: %s\n", err.Error())
	}
}

func readConfig(configFilename string) (configuration, error) {
	var config configuration

	configFile, err := os.Open(configFilename)
	if err != nil {
		return config, fmt.Errorf("Error opening config file: %s\n", err.Error())
	}

	// parse the config file data to configParser struct
	jsonParser := json.NewDecoder(configFile)
	if err = jsonParser.Decode(&config); err != nil {
		return config, fmt.Errorf("Error parsing config file. Please check format of the file!\nParse Error: %s\n", err.Error())
	}
	return config, nil
}

func startMonitoring(config configuration, logFileName string) {
	var err error

	// setup different notification clients
	notify.AddNew(config.Notifications)
	// Send test notifications to all the notification clients
	notify.SendTestNotification()

	// Create unique ids for each request date given in config file
	reqs, ids := validateAndCreateIdsForRequests(config.Requests)

	// Set up and initialize databases

	err = database.ParseDBConfig(config.Database)
	if err != nil {
		fmt.Println(err)
		os.Exit(3)
	}
	database.Initialize(ids, config.NotifyWhen.MinResponseCount, config.NotifyWhen.ErrorCount)

	// Initialize and start monitoring all the apis
	requests.RequestsInit(reqs, config.Concurrency)
	requests.StartMonitoring()

	logger.EnableLogging(logFileName)

	// Just to check StatusOk is running or not
	http.HandleFunc("/", statusHandler)

	usedPort := config.Port
	if usedPort == 0 {
		usedPort = DefaultPort
	}

	localAddress := fmt.Sprintf(":%d", usedPort)
	err = http.ListenAndServe(localAddress, nil)
	if err != nil {
		panic(err)
	}
}

// Currently just tells status ok is running
// Planning to display useful information in future
func statusHandler(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "StatusOk is running.\n Maybe display other useful information in further releases...?")
}

// Tells whether a file exits or not
func fileExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func logFilePathValid(name string) bool {
	f, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return false
	}
	defer f.Close()

	return true
}

// checks whether each request in config file has valid data
// Creates unique ids for each request using math/rand
func validateAndCreateIdsForRequests(reqs []requests.RequestConfig) ([]requests.RequestConfig, map[int]int64) {
	source := rand.NewSource(time.Now().UnixNano())
	random := rand.New(source)

	// an array of ids used by database pacakge to calculate mean response time and send notifications
	ids := make(map[int]int64)

	// an array of new requests data after updating the ids
	newreqs := make([]requests.RequestConfig, 0)

	for i, requestConfig := range reqs {
		validateErr := requestConfig.Validate()
		if validateErr != nil {
			fmt.Printf("Invalid Request data in config file for Request #%d: %s\n", i, requestConfig.Url)
			fmt.Printf("Error: %s\n", validateErr.Error())
			os.Exit(3)
		}

		// Set a random value as id
		randInt := random.Intn(1000000)
		ids[randInt] = requestConfig.ResponseTime
		requestConfig.SetId(randInt)
		newreqs = append(newreqs, requestConfig)
	}

	return newreqs, ids
}
