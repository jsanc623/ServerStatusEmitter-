package main

import (
	"collector"
	"encoding/json"
	"fmt"
	"helper"
	"log"
	"os"
	"sse"
	"time"
)

var (
	URL          = "http://mothership.serverstatusmonitoring.com"
	URIRegister  = "/register"
	URICollector = "/collector"
	URIStatus    = "/status"

	Hostname  = ""
	IPAddress = ""

	LogFile           = "/var/log/sphire-sse.log"
	ConfigurationFile = "/etc/sse/sse.conf"
	Configuration     = new(Configuration)

	CollectFrequencySeconds = 1 // Collect a snapshot and store in cache every X seconds
	ReportFrequencySeconds  = 2 // Report all snapshots in cache every Y seconds

	CPU     collector.CPU     = collector.CPU{}
	Disks   collector.Disks   = collector.Disks{}
	Memory  collector.Memory  = collector.Memory{}
	Network collector.Network = collector.Network{}
	System  collector.System  = collector.System{}

	Version = "1.0.1"
)

/*
 Configuration struct is a direct map to the configuration located in the configuration JSON file.
*/
type Configuration struct {
	Identification struct {
		AccountID        string `json:"account_id"`
		OrganizationID   string `json:"organization_id"`
		OrganizationName string `json:"organization_name"`
		MachineNickname  string `json:"machine_nickname"`
	} `json:"identification"`
}

func main() {
	// Define the global logger
	logger, err := os.OpenFile(LogFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	HandleError(err)
	defer logger.Close()
	log.SetOutput(logger)

	// Load and parse configuration file
	file, err := os.Open(ConfigurationFile)
	HandleError(err)
	err := json.NewDecoder(file).Decode(Configuration)
	HandleError(err)

	var status helper.Status = helper.Status{}
	var status_result bool = status.CheckStatus(URL + URIStatus)
	if status_result == false {
		HandleError(error("Mothership unreachable. Check your internet connection."))
	}

	// Perform system initialization
	var server_obj sse.Server = sse.Server{}
	var server *sse.Server = &sse.Server{}
	server, ipAddress, hostname, version, err = server_obj.Initialize()
	HandleError(err)

	// Perform registration
	body, err := sse.Register(map[string]interface{}{
		"configuration":     Configuration,
		"mothership_url":    URL,
		"register_uri":      URIRegister,
		"version":           Version,
		"collect_frequency": CollectFrequencySeconds,
		"report_frequency":  ReportFrequencySeconds,
		"hostname":          Hostname,
		"ip_address":        IPAddress,
		"log_file":          LogFile,
		"config_file":       ConfigurationFile,
	}, URL+URIRegister+"/"+Version)
	if err != nil {
		HandleError(error("Unable to register this machine" + string(body)))
	}

	// Set up our collector
	var counter int = 0
	var snapshot sse.Snapshot = sse.Snapshot{}
	var cache sse.Cache = sse.Cache{
		AccountId:        Configuration.Identification.AccountID,
		OrganizationID:   Configuration.Identification.OrganizationID,
		OrganizationName: Configuration.Identification.OrganizationName,
		MachineNickname:  Configuration.Identification.MachineNickname,
		Version:          Version,
		Server:           server}

	ticker := time.NewTicker(time.Duration(CollectFrequencySeconds) * time.Second)

	for {
		<-ticker.C // send the updated time back via the channel

		// reset the snapshot to an empty struct
		snapshot = sse.Snapshot{}

		// fill in the Snapshot struct and add to the cache
		cache.Node = append(cache.Node, snapshot.Collector())
		counter++

		if counter > 0 && counter%ReportFrequencySeconds == 0 {
			cache.Sender(URL + URICollector)
			cache.Node = nil // Clear the Node Cache
			counter = 0
		}
	}

	return

}

func HandleError(err error) {
	if err != nil {
		log.Println(helper.Trace(err, "ERROR"))
		fmt.Println(err, "ERROR")
		os.Exit(1)
	}
}
