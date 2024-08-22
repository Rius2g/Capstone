package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gin-gonic/gin"
    "github.com/ethereum/go-ethereum/accounts/abi"
    "strings"
    
)


const (
    infuraURL       = "https://mainnet.infura.io/v3/YOUR_INFURA_PROJECT_ID"
    contractAddress  = "0xYourContractAddress"
    contractABI      = `[{"constant":true,"inputs":[],"name":"getValue","outputs":[{"name":"","type":"uint256"}],"payable":false,"stateMutability":"view","type":"function"}]`
)

type Contract struct {
    contract abi.ABI 
    address common.Address
    client *ethclient.Client
}


var client *ethclient.Client
var contractInstance *Contract
func main() {
	// Connect to an Avalanche node
    var err error
	client, err = ethclient.Dial("https://api.avax.network/ext/bc/C/rpc")
	if err != nil {
		log.Fatal(err)
	}


    parsedABI, err := abi.JSON(strings.NewReader(contractABI))
    if err != nil {
        log.Fatalf("Failed to parse contract ABI: %v", err)
    }

    contractAddress := common.HexToAddress(contractAddress)

    contractInstance = &Contract{contract: parsedABI, address: contractAddress, client: client}


	// Create a new Gin router
    startServer()
}

func startServer(){
    // Create a new Gin router
    r := gin.Default()

    r.GET("/chainID", getChainID)
    r.GET("/blockNumber", getBlockNumber)

    r.POST("/postToChain", postToChain)
    r.GET("/getFromChain", getFromChain)

    // Run the server
    r.Run(":8080")
}


func getChainID(c *gin.Context){
    // Get the chain ID
    chainID, err := client.ChainID(c)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{"chainID": chainID.String()})
}


func getFromChain(c *gin.Context){
}


func postToChain(c *gin.Context){
    //get the data from the request and post to chain 

    encryptedData := c.PostForm("data")
    releaseTime := c.PostForm("releaseTime")
    decryptionKey := c.PostForm("decryptionKey")
    owner := c.PostForm("owner")

    fmt.Println(encryptedData, releaseTime, decryptionKey, owner)
    // Post the data to the postToChain

    c.JSON(http.StatusOK, gin.H{"message": "Post to chain"})
}


func getBlockNumber(c *gin.Context){
	// Get the latest block number
		blockNumber, err := client.BlockNumber(c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"blockNumber": blockNumber})

}
