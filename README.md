#Logger

A simple Go based logging tool for HTTP requests

##Dependencies

- Go v1.1.2+ 
- Facebook Grace package (https://github.com/facebookgo/grace)

##Installation

- `$ sudo apt-get install golang`
- `$ git clone https://github.com/MindGeekOSS/Logger.git`
- `$ go get github.com/facebookgo/grace`
- `$ go build logger.go`

## Starting the process

`$ ./logger -c logger.conf`


##Configuration details

`logger_address=[X.X.X.X:Y]`: 
	- ip and port on which to listen to for the logger (default 0.0.0.0:80)

`debug=[0|1]`: 
	- enable debug mode (0|1, default 0)

`log_directory=[DIR]`: 
	- directory in which the log files will be generated (default logs/, relative to binary directory)

`num_workers=[N]`: 
	- number of workers to handle logging (default = 16)

`buffer_capacity=[N]`:
	- text buffer capacity in bytes for each LogWorker (default = 256)

`force_fsync=[0|1]`:
	- explicitly issue an fsync when writting data to file (default = 0)
`enable_ssl=[0|1]`:
	- enables https mode (not yet implemented)

`enable_stats=[0|1]`:
	- enable the statistics reporting (default = 1, feature not fully tested and/or stable yet)

`stats_address=[X.X.X.X:Z]`:
	- ip and port on which to listen to for the stats reporting (default 0.0.0.0:88)

`generate_udid=[0|1]`:
	- generate a udid and store it in cookie (default = 0)

`cookie_domain=[DOMAIN]`:
	- domain on which to send the cookie for the UDID (only used if generate_udid = 1)

`dump_to_graphite=[0|1]`:
	- Enable stats dumping to graphite (not yet implemented)

`graphite_host=[IP]`:
	- Host on which graphite is listening (not yet implemented)

`graphite_port=[PORT]]`:
	- Port on which Graphite is listening (not yet implemented)


## Considerations

Depending on the amount of requests per second (RPS) you're expcting, you'll have to tweek the num_workers, buffer_capacity, and force_fsync parameters.  Keep in mind that you'll be able to acheive a greater throughput the larger your 
num_workers and buffer_capacity values are as you'll essentially be writing to disk less often.

Note that although they are not yet configurable, the log event and pending write channels (can be modified within the logger.go code) can also be increased and decreased depending on your need.  Their values have been set high initially so that they can collect a relatively high number of items before blocking any Log Workers.


## TODO / Future Features:

- Implement SSL feature for both logging and stats reporting
- Implement reporting of realtime data to Graphite
- Cleanup the code by sperating utility functions and structs into their own seperate modules
- Complete stats section (CPU usage, current RPS, uptime, connections handled, etc)
- Test modificaitons to specific sysctl kernel values to verify any potential improvments in performance
- Perform an official stress test along with results which include RPS, logfile size, CPU usage, etc.


## Acknowledgements 

Parts of this project were influenced by some of Karl Seguin's work relating to concurency in Go.
Special thanks to Scott Cameron for feedback and advice related system kernel tuning and load testing.


## Creator

Alain Lefebvre <alain.lefebvre 'at' mindgeek.com>


## License

Copyright 2014 MindGeek, Inc.

Covered under the GNU GPL v3.0 lisense.
