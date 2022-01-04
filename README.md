# Metalife Pub installation instructions and functional description

  Metalife Pub is a P2P communication service built using the Scuttlebutt protocol. Its main purpose is to provide remote Peer node interactive connections and to collect like data that the authors publicly released in Pub ,which will report to super nodes for incentives. Since Pub is used as a node for Mediate Transfer, it is necessary to install the corresponding Photon node for payment functions at the same time. The following are installation instructions and related functional descriptions.

## Pub Installation  

### Pub system environment requirements

   Golang version 1.16.6 or higher
  
  The available running memory needs to reach 2G and above, and the available disk space needs to reach 50G and above.

The above configuration is the basic configuration, and the hardware configuration can be further upgraded for Pubs with storage requirements in the future.

### Install the pub

 This version of Pub is written in Go. We recommend that users pre-install the latest version of Go. And further execute the following script to install pub.

```go
git clone https://github.com/MetaLife-Foundation/MetalifePub
cd MetalifePub 
go install ./cmd/go-sbot
go install ./cmd/sbotcli
```

   **Instruction**
    Just pull the code on the master branch. This branch supports ebt-mutlifromat by default. It should be noted that a photon node needs to be run on the server where Pub is installed, and please enter the parameter configuration file **/restful/params/config.go**. The parameter of PhotonAddress(in line 46)  need to be replaced by the address of the photon node running on the same machine (this account needs to ensure that the number of SMT is sufficient, the pub node will automatically establish the channel with each connected client account, and deposit 0.1smt to the channel)

 **Make sure the  go-sbot and sbotcli are in the $PATH**
1.We need to manually create a folder **. ssb-go** on the user's home directory of the server.

```go
    mkdir -p $HOME/.ssb-go
```

2.Run

```go
nohup go-sbot > ~/ssb/go-sbot.log &
nohup sbotcli > ~/ssb/sbotcli.log &
```

The running log will be saved in **server.log** and **sbotcli.log**.

### Function Description

**go-sbot**
The pub server program ,which will provide client access and message log synchronization services that follow the ssb protocol.
**sbotcli**
The pub message monitoring service provides:
1、Get the eth address corresponding to the customer ID:

```go
GET http://{your server ip}:18008/ssb/api/node-address
```

2、Get Like Statistics
```go
GET http://{your server ip}:18008/ssb/api/likes
```

3、Channel establishment and pre-deposit service  

**Notice**Here is the spectrum ,TokenAddress=”0x6601F810eaF2fa749EEa10533Fd4CC23B8C791dc”
4、Extension: The monitoring service has mastered all the likes and other types of messages, and will provide details of the like-source of the specific likes in the follow-up.
