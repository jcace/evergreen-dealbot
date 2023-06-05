package main

import (
	"github.com/caarlos0/env/v6"
	"github.com/joho/godotenv"

	log "github.com/sirupsen/logrus"
)

type EvergreenDealbotConfig struct {
	Lotus struct {
		FullNodeApiInfo   string `env:"FULLNODE_API_INFO,notEmpty"`
		MinerApiInfo      string `env:"MINER_API_INFO,notEmpty"`
		BoostUrl          string `env:"BOOST_URL,notEmpty"`
		BoostAuthToken    string `env:"BOOST_AUTH_TOKEN,notEmpty"`
		MaxRetrievalPrice string `env:"MAX_RETRIEVAL_PRICE" envDefault:"0"`
		RetrievalTimeout  uint   `env:"RETRIEVAL_TIMEOUT_MINUTES" envDefault:"10"`
		MinPieceSize      int64  `env:"MIN_PIECE_SIZE" envDefault:"1073741824"`
	}

	Evergreen struct {
		DealRequeryInterval          uint `env:"AVAILABLE_DEAL_QUERY_INTERVAL_MINUTES" envDefault:"2"`
		MaxConcurrentRetrievalsPerSp uint `env:"MAX_CONCURRENT_RETRIEVALS_PER_SP" envDefault:"2"`
	}

	Common struct {
		MaxThreads          uint   `env:"MAX_THREADS" envDefault:"4"`
		CarLocationLongterm string `env:"CAR_LOCATION_LONGTERM" envDefault:"/tmp"`
		CarLocationDownload string `env:"CAR_LOCATION_DOWNLOAD" envDefault:"/tmp"`
		LogDebug            bool   `env:"DEBUG" envDefault:"false"`
		LogFileLocation     string `env:"LOG_FILE_LOCATION" envDefault:""`
	}
}

func InitConfig() EvergreenDealbotConfig {
	godotenv.Load()
	var cfg EvergreenDealbotConfig

	if err := env.Parse(&cfg); err != nil {
		log.Fatalf("Error parsing config: %+v\n", err)
	}

	log.Debugf("Config Parsed: %+v \n", cfg)

	return cfg
}
