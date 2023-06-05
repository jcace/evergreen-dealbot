# Readme


## Prerequisites
You need everything to be able to build [ filecoin-ffi ]( https://github.com/filecoin-project/filecoin-ffi ). Ensure Rust is installed:
```bash
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
```


## Build and installation
1. Clone this repo
2. `make build`
3. Once successful, `make install` to copy it to your PATH
4. Set up an environment variable file, refer to `sample.env` for all the required variables
5. Create service to run the dealbot:

### Sample Service file

Create this file in `/etc/systemd/system/evergreen-dealbot.service`

Make sure to change the `EnvironmentFile` variable to match the path to the `.env` file created above. 
```bash
[Unit]
Description=Evergreen Dealbot
After=network-online.target
Requires=network-online.target

[Service]
ExecStart=/usr/local/bin/evergreen-dealbot
EnvironmentFile=/home/filecoin/feeds/evergreen/evergreen-dealbot.env
User=filecoin
Group=filecoin
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

6. Start up the service `systemctl start evergreen-dealbot`
7. View logs `tail -F /var/log/evergreen-dealbot.log`

# Configuration
See `config.go` for the environment variables that must be configured for the application to work. 
You can also find an example configuration in `sample.env`.


# Developer Notes

## Install for dev
https://github.com/filecoin-project/filecoin-ffi#go-get
## Running locally for dev
`go run .`
## M1 Macbook Fix
On macos, you need to do `export LIBRARY_PATH=/opt/homebrew/lib`
[export LIBRARY_PATH=/opt/homebrew/lib](https://github.com/application-research/estuary#guide-for-missing-hwloc-on-m1-macs)

### Example "available deal"
```
/**
	 * {
          "source_type": "Filecoin",
          "provider_id": "f01278",
          "deal_id": 6452884,
          "original_payload_cid": "QmSaE9PTE1HRUrQHsy1kxanzLJdPQskvFquQBMVyqbx467",
          "deal_expiration": "2023-11-09T14:54:00Z",
          "is_filplus": true,
          "sector_id": null,
          "sector_expires": null,
          "sample_retrieve_cmd": "lotus client retrieve --provider f01278 --maxPrice 0 --allow-local --car 'QmSaE9PTE1HRUrQHsy1kxanzLJdPQskvFquQBMVyqbx467' $(pwd)/baga6e~g3ufyaoy__QmSaE9~Vyqbx467.car"
        },
*/
```

### Example Retrieval Command
```
lotus client retrieve --provider f01278 --maxPrice 0 --allow-local --car 'QmaaicnpT7Jes7QeLxrbFmpppAjouHd2y3RrvQ4kZibCQY' $(pwd)/baga6e~noumj6jq__Qmaaic~4kZibCQY.car
```

