# Metalife Server installation instructions and functional description

  Metalife Server is a P2P communication service built using the Scuttlebutt protocol. Its main purpose is to provide remote Peer node interactive connections and to collect like data that the authors publicly released in Pub ,which will report to super nodes for incentives. Since Pub is used as a node for Mediate Transfer, it is necessary to install the corresponding Photon node for payment functions at the same time. The following are installation instructions and related functional descriptions.

## Server Installation  

### Server environment requirements

   Golang version 1.16.6 or higher
  
  The available running memory needs to reach 2G and above, and the available disk space needs to reach 50G and above.

The above configuration is the basic configuration, and the hardware configuration can be further upgraded for Pubs with storage requirements in the future.

### Install the Server

 This version of Server is written in Go. We recommend that users pre-install the latest version of Go. And further execute the following script to install Server.

```bash
git clone https://github.com/MetaLife-Foundation/MetalifePub
cd MetalifePub 
go install ./cmd/metalifeserver
```

   **Instruction**
    Just pull the code on the master branch. This branch supports ebt-mutlifromat by default. It should be noted that a photon node needs to be run on the server where Server is installed, and please enter the parameter configuration file **/restful/params/config.go**. The parameter of PhotonAddress(in line 46)  need to be replaced by the address of the photon node running on the same machine (this account needs to ensure that the number of SMT is sufficient, the pub node will automatically establish the channel with each connected client account, and deposit 0.1smt to the channel)

 **Make sure the  go-sbot and sbotcli are in the $PATH**
 
1.We need to manually create a folder **. ssb-go** on the user's home directory of the server.

```bash
    mkdir -p $HOME/.ssb-go
```
2.create a file named 'secret' and input the ssb-server's private key file,like this:

```bash
{
  "curve": "ed25519",
  "public": "M2nU6wM+F17J0RSLXP05x3Lag8jGv3F3LzHMjh72coE=.ed25519",
  "private": "6t1JnzJz0M4imTUUeoQuYdNnFPcZ78IwwRsjgQN1kMcdmdTrAz4XXsnRFItc/TnHcreDaMa/cXbtMcyOHvZygQ==.ed25519",
  "id": "@M2nU6wM+F17J0RSLXP05x3Lag8jGv3F3LzHMjh72coE=.ed25519"
}
```

3.Run(The database will be automatically created after the program runs. The type is sqlite)

```bash
nohup sbotcli > log &
```

The running log will be saved in **log**.

4.Some key operating parameters are located in /MetalifePub/restful/params/config.go

```bash
/MetalifePub/restful/params/config.go
```

### Function Description

**metalifeserver**
The MetaLife server program ,which will provide client access and message log synchronization services that follow the ssb protocol.
The pub message monitoring service provides:

1.Get all ssb-client information on a pub:

```bash
GET http://{ssb-server-public-ip}:18008/ssb/api/node-info

```
Response e.g:
```json
	{
        "error_code": 0,
        "error_message": "SUCCESS",
        "data": [
            {
                "client_id": "@C49GskstTGIrvYPqvTk+Vjyj23tD0wbCSkvX7A4zoHw=.ed25519",
                "client_Name": "beefi",
                "client_alias": "9527",
                "client_bio": "SG",
                "client_eth_address": "0xce92bddda9de3806e4f4b55f47d20ea82973f2d7"
            },
            {
                "client_id": "@eVs235wBX5aRoyUwWyZRbo9r1oZ9a7+V+wEvf+F/MCw=.ed25519",
                "client_Name": "an-Pub1",
                "client_alias": "",
                "client_bio": "SG",
                "client_eth_address": ""
            }
        ]
    }
```

2.Get someone-ssb-client information on a pub:

```bash
GET http://{ssb-server-public-ip}:18008/ssb/api/node-info
```
Body:
```json
{
    "client_id":"@C49GskstTGIrvYPqvTk+Vjyj23tD0wbCSkvX7A4zoHw=.ed25519"
}
```
Response e.g:
```json
	{
        "error_code": 0,
        "error_message": "SUCCESS",
        "data": [
            {
                "client_id": "@C49GskstTGIrvYPqvTk+Vjyj23tD0wbCSkvX7A4zoHw=.ed25519",
                "client_Name": "beefi",
                "client_alias": "",
                "client_bio": "SG",
                "client_eth_address": "0xce92bddda9de3806e4f4b55f47d20ea82973f2d7"
            }
        ]
    }
```

3.The ssb-client register with metalifeserver the ETH address used to receive MetaLife's reward:


```bash
Post http://{ssb-server-public-ip}:18008/ssb/api/id2eth
```
Body:
```json
{
    "client_id":"@C49GskstTGIrvYPqvTk+Vjyj23tD0wbCSkvX7A4zoHw=.ed25519",
    "client_eth_address":"0xce92bddda9de3806e4f4b55f47d20ea82973f2d7"
}
```
Response e.g:
```json
{
    "error_code": 0,
    "error_message": "SUCCESS",
    "data": "success"
}
```

4.Get 'Like' Statistics of all on pub

```bash
GET http://{ssb-server-public-ip}:18008/ssb/api/likes
```
Response e.g:
```json
	{
        "error_code": 0,
        "error_message": "SUCCESS",
        "data": {
            "@C49GskstTGIrvYPqvTk+Vjyj23tD0wbCSkvX7A4zoHw=.ed25519": {
                "client_id": "@C49GskstTGIrvYPqvTk+Vjyj23tD0wbCSkvX7A4zoHw=.ed25519",
                "laster_like_num": 7,
                "client_name": "beefi",
                "client_eth_address": "0xce92bddda9de3806e4f4b55f47d20ea82973f2d7"
            },
            "@eVs235wBX5aRoyUwWyZRbo9r1oZ9a7+V+wEvf+F/MCw=.ed25519": {
                "client_id": "@eVs235wBX5aRoyUwWyZRbo9r1oZ9a7+V+wEvf+F/MCw=.ed25519",
                "laster_like_num": 0,
                "client_name": "an-Pub1",
                "client_eth_address": ""
            }
        }
    }
```

5.Get 'Like' Statistics of someone-ssb-client on pub


```bash
GET http://{ssb-server-public-ip}:18008/ssb/api/likes
```
Body:
```json
{
    "client_id":"@C49GskstTGIrvYPqvTk+Vjyj23tD0wbCSkvX7A4zoHw=.ed25519"
}
```
Response e.g:
```json
	{
        "error_code": 0,
        "error_message": "SUCCESS",
        "data": {
            "@C49GskstTGIrvYPqvTk+Vjyj23tD0wbCSkvX7A4zoHw=.ed25519": {
                "client_id": "@C49GskstTGIrvYPqvTk+Vjyj23tD0wbCSkvX7A4zoHw=.ed25519",
                "laster_like_num": 7,
                "client_name": "beefi",
                "client_eth_address": "0xce92bddda9de3806e4f4b55f47d20ea82973f2d7"
            }
        }
    }
```

3.Channel establishment and pre-deposit service  
After receiving the ETH address registration message, the  MetaLife server will actively establish a channel with the client to obtain rewards , on Spectrum Main Chain.

**Notice**Here is the spectrum ,TokenAddress=”0x6601F810eaF2fa749EEa10533Fd4CC23B8C791dc”

4.Extension: The monitoring service has mastered all the likes and other types of messages, and will provide details of the like-source of the specific likes in the follow-up.
