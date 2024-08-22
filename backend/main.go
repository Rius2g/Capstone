package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ethereum/go-ethereum/ethclient"
)

var client *ethclient.Client
func main() {
	// Connect to an Avalanche node
    var err error
	client, err = ethclient.Dial("https://api.avax.network/ext/bc/C/rpc")
	if err != nil {
		log.Fatal(err)
	}

	// Create a new Gin router
	r := gin.Default()

    r.GET("/chainID", getChainID)
    r.GET("/blockNumber", getBlockNumber)


	// Define a route to get the latest block number

	// Define a route to get the chain ID

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


func getBlockNumber(c *gin.Context){
	// Get the latest block number
		blockNumber, err := client.BlockNumber(c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"blockNumber": blockNumber})

}
