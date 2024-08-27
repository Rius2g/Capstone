package main

import (
	chord "Backend/chord"
	h "Backend/helper"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type Contract struct {
	contract *bind.BoundContract
	address  common.Address
	client   *ethclient.Client
}

const (
	infuraURL       = "https://api.avax.network/ext/bc/C/rpc"
	contractAddress = "0xYourContractAddress"
)

var (
	client           *ethclient.Client
	self             *h.Node
	contractInstance *Contract
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var wg sync.WaitGroup

func HandleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()

	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			log.Println(err)
			return
		}
		if err := conn.WriteMessage(messageType, p); err != nil {
			log.Println(err)
			return
		}
	}
}

func StartChordHandler(c *gin.Context) {
	keySize, err := strconv.Atoi(c.Request.URL.Query().Get("KeySize"))
	h.HandleError(err, "KeySize not an int", false)
	self.ChordSize = (1 << keySize)
	self = chord.CreateChord(self)
	c.String(http.StatusOK, fmt.Sprintf("Chord of size %d created by node %d.\n", self.ChordSize, self.Id))
}

func FindNodeId(c *gin.Context) {
}

func main() {
	// Connect to an Avalanche node
	var err error

	abiFilePath := "contract_abi.json"
	abiJSON, err := os.ReadFile(abiFilePath)
	if err != nil {
		log.Fatalf("Failed to read ABI file: %v", err)
	}

	// Parse the ABI
	parsedABI, err := abi.JSON(strings.NewReader(string(abiJSON)))
	if err != nil {
		log.Fatalf("Failed to parse ABI: %v", err)
	}

	client, err = ethclient.Dial("https://api.avax.network/ext/bc/C/rpc")
	if err != nil {
		log.Fatal(err)
	}

	contractAddress := common.HexToAddress(contractAddress)

	boundContract := bind.NewBoundContract(contractAddress, parsedABI, client, client, client)

	contractInstance = &Contract{
		contract: boundContract,
		address:  contractAddress,
		client:   client,
	}

	query := ethereum.FilterQuery{
		Addresses: []common.Address{contractAddress},
	}

	logs := make(chan types.Log)
	sub, err := client.SubscribeFilterLogs(context.Background(), query, logs)
	if err != nil {
		log.Fatalf("Failed to subscribe to logs: %v", err)
	}

	go startServer()
	for {
		select {
		case err := <-sub.Err():
			log.Fatalf("Subscription error: %v", err)
		case vLog := <-logs:
			//if node is seed node then handle the event
			if self.Seed { //how to abort this function if the node is no longer seed?
				go handleEvent(vLog, parsedABI)
			}
		}
	}
}

/*
* Function to handle the RTT of the node
* @param c: The context of the Request
* @param predecessor: The predecessor of the nodes
* @param RTT: The RTT of the nodes
 */
func RTTPredecessor(c *gin.Context) {
	var predecessor h.Node
	err := c.BindJSON(&predecessor)
	if err != nil {
		fmt.Println("Could not bind the JSON")
		return
	}

	RTT, err := strconv.ParseInt(c.Request.URL.Query().Get("RTT"), 10, 64)
	if err != nil {
		fmt.Println("Could not parse the RTT")
		return
	}

	//Set up the closest node to the current node in terms of RTT
	//how to determine the closest node in terms of RTT and be sure that every node has different "closest" nodes
	if predecessor.Id == self.Id {
		c.String(http.StatusOK, "Already predecessor")
		return
	}
	if predecessor.Id == self.Neighbours.Predecessor.Id {
		c.String(http.StatusOK, "Already predecessor")
		return
	}

	if int(RTT) > self.RTTNeighbours.RTTPredecessor.RTTTime { //this means this node has a pre that is closer in terms of RTT
		if predecessor.Id != self.Id { //have to check if the predecessor is not the current node or the same as the current node
			c.String(http.StatusOK, "Already predecessor")
		} else {
			c.String(http.StatusOK, "Predecessor set successfully")
		}
	}
	self.RTTNeighbours.RTTPredecessor.RTTNode = &predecessor
	c.String(http.StatusOK, "Not set as predecessor")

}

func GetBestRTTAddress(RttTimes *[]int64, NodeAddresses *[]string) string {
	// Find the best RTT time and the address of the node with the best RTT
	var bestRTT int64 = math.MaxInt64
	bestRTTAddress := ""

	for i, rt := range *RttTimes {
		if rt < bestRTT {
			bestRTT = rt
			bestRTTAddress = (*NodeAddresses)[i]
		}
	}

	// Remove the best RTT from the list of RTTs and the address of the node with the best RTT
	// This is to ensure that we do not keep on sending messages to the same node

	// Find the index of the best RTT to remove it
	for i, rt := range *RttTimes {
		if rt == bestRTT {
			// Remove the best RTT and corresponding address
			*RttTimes = append((*RttTimes)[:i], (*RttTimes)[i+1:]...)
			*NodeAddresses = append((*NodeAddresses)[:i], (*NodeAddresses)[i+1:]...)
			break
		}
	}

	return bestRTTAddress
}

/*
* This is used to set a Seeds succsessor node for data broadcasting
* Also used if a succsessor node gets a new pred
 */
func NoLongerPredecessor(LastSuccessorAddress string, NodeAddresses []string) {
	RttTimes := []int64{}
	for _, address := range NodeAddresses {
		if address == LastSuccessorAddress {
			continue
		}
		rt := chord.RTT(address)
		if rt == -1 {
			fmt.Println("Could not ping the node")
			continue
		}
		RttTimes = append(RttTimes, rt)
	}

	body, err := json.Marshal(self)
	if err != nil {
		fmt.Println("Could not marshal the body")
		return
	}

	//now get the best RTT and the address and try over with different nodes until we get a successful response
	for {
		BestRttNode := GetBestRTTAddress(&RttTimes, &NodeAddresses)
		postURL := BestRttNode + "/setPredecessor"
		resp, err := http.Post(postURL, "application/json", bytes.NewBuffer(body))
		if err != nil {
			fmt.Println("Could not send the post request")
			return
		}
		defer resp.Body.Close()

		//read the response from the new successor
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("Could not read the response")
			return
		}

		fmt.Println(string(respBody)) //need to check if the response is successful, if it is then we can break out of the loop
		if resp.StatusCode == 200 {
			//check the resp body to see if the message is successful
			if strings.Contains(string(respBody), "Pred set successfully") {
				break
			}
		}
	}

}

func NoLongerPredecessorHandler(c *gin.Context, LastSuccessorAddress string, NodeAddresses []string) {
	NoLongerPredecessor(LastSuccessorAddress, NodeAddresses)
	c.JSON(http.StatusOK, gin.H{"message": "New predecessor set successfully"})

}

func SetupRTTs(NodeAddresses []string) { //probably need more than 1 succsessor, have to do some math on this to see how many succsessors we need on seeds and also how many seeds, what is best for scalability?
	//get a list of all the nodes in the network and ping them to get the RTT's
	//this will be done by the seed nodes
	//the seed nodes will then send the RTT's to the nodes in the network
	//get all the nodes in the network to find the closest nodes in terms of RTT
	//find the closest nodes in terms of RTT, send it a message to say this is the closest node in terms of RTT
	NoLongerPredecessor(self.Address, NodeAddresses) //this is called to set the successor of this node
}

/*
* Function to sned to the node we want to leave to make it leave
* @param c is the context of the get request.
* @returns: Responds to the request
 */
func NodeLeave(c *gin.Context) {
	go chord.NodeLeft(self)
	c.String(200, "Node %d Left\n", self.Id)
}

/*
* Function to handle a new node joining the network
* @param c is the context of the get request.
* @returns: Responds to the request
 */
func NodeJoin(c *gin.Context) {

	//the node that receives this must join the network of Nprime == HOST:PORT in body of the post request (sent to lonely node)
	node2join := c.Request.URL.Query().Get("nprime")
	wg.Add(1)
	go func() {
		chord.NodeJoin(self, node2join)
		wg.Done()
	}()
	wg.Wait()
	c.String(200, "Node Joined\n")
}

/*
* Return the chord size to the new node in the chord for it to hash with
 */
func getInfo(c *gin.Context) {
	log.Printf("sending info from %s\n", self.Address)

	log.Printf("sending info from %d\n", self.Id)
	c.String(http.StatusOK, strconv.Itoa(self.ChordSize))
}

func Ping(c *gin.Context) {
	// Get the RTT of the node
	c.JSON(http.StatusOK, gin.H{})
}

func startServer() {
	// Create a new Gin router
	r := gin.Default()
	r.Use(cors.Default())

	r.GET("/chainID", getChainID)
	r.GET("/blockNumber", getBlockNumber)
	r.GET("/ws", HandleWebSocket)
	r.GET("get_locked_data", getLockedData)
	r.GET("/RTTPredecessor", RTTPredecessor)
	r.GET("/RTT", Ping)
	r.GET("/getFromChain", getFromChain)
	r.GET("/node-id/:id", FindNodeId)
	r.POST("/start", StartChordHandler)

	r.POST("/postToChain", postToChain)

	// Run the server
	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}

func getLockedData(c *gin.Context) {
	//interact with the contract and get the data
	callOpts := &bind.CallOpts{
		Context: context.Background(),
	}

	var data *big.Int
	var out []interface{}

	out = append(out, &data)

	err := contractInstance.contract.Call(callOpts, &out, "returnStoredData")
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to call contract: %v", err))
		return
	}

	c.String(http.StatusOK, fmt.Sprintf("Data: %v", data))

}

func getChainID(c *gin.Context) {
	// Get the chain ID
	chainID, err := client.ChainID(c)
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to get chain ID: %v", err))
		return
	}

	c.String(http.StatusOK, fmt.Sprintf("Chain ID: %v", chainID))
}

func getFromChain(c *gin.Context) {
}

func generateKeyPair() (*rsa.PrivateKey, *rsa.PublicKey, error) {
	// Generate a public, private key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	publicKey := &privateKey.PublicKey

	return privateKey, publicKey, nil
}

func encryptData(data string) (string, string, error) {

	// create a public, privatekey pair and encrypt the encryptData
	privateKey, publicKey, err := generateKeyPair()
	if err != nil {
		return "", "", err
	}

	// Encrypt the encryptData

	encryptedData, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, publicKey, []byte(data), nil)
	if err != nil {
		return "", "", err
	}

	return string(encryptedData), string(privateKey.D.Bytes()), nil

}

func decryptData(encryptedData, privateKeyPEM string) (string, error) {
	// Decode the base64 encrypted data
	encryptedDataBytes, err := base64.StdEncoding.DecodeString(encryptedData)
	if err != nil {
		return "", err
	}

	// Decode the PEM private key
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		return "", errors.New("failed to decode PEM block containing private key")
	}

	// Parse the private key
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return "", err
	}

	// Decrypt the data
	decryptedData, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, privateKey, encryptedDataBytes, nil)
	if err != nil {
		return "", err
	}

	return string(decryptedData), nil
}

func handleEvent(vLog types.Log, parsedABI abi.ABI) {
	switch vLog.Topics[0].Hex() {
	case parsedABI.Events["PushEncryptedData"].ID.Hex():
		var ContractData struct {
			EncryptedData string
			Owner         string
			DataName      string
		}
		err := parsedABI.UnpackIntoInterface(&ContractData, "PushEncryptedData", vLog.Data)
		if err != nil {
			log.Println("Error unpacking PushEncryptedData event:", err)
			return
		}
		log.Printf("Encrypted data received: Encrypted data: %s\n", ContractData.EncryptedData)

	case parsedABI.Events["PushPrivateKey"].ID.Hex():
		var ContractData struct {
			PrivateKey string
			Owner      string
			DataName   string
		}
		err := parsedABI.UnpackIntoInterface(&ContractData, "PushPrivateKey", vLog.Data)
		if err != nil {
			log.Println("Error unpacking PrivateKey event:", err)
			return
		}
		log.Printf("Pushed private key event: Private key: %s\n", ContractData.PrivateKey)

	default:
		log.Printf("Unhandled event: %s\n", vLog.Topics[0].Hex())
	}
}

func postToChain(c *gin.Context) {
	//get the data from the request and post to chain

	Data := c.PostForm("data")
	releaseTime := c.PostForm("releaseTime")
	owner := c.PostForm("owner")

	//encrypt the data and post to postToChain
	sha256Hash := sha256.New()
	sha256Hash.Write([]byte(Data))
	hashedData := sha256Hash.Sum(nil)

	//need to hash the data so it has a key to be stored in the contract

	fmt.Println(Data, releaseTime, owner)

	encryptedData, privateKey, err := encryptData(Data)
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to encrypt data: %v", err))
		return
	}

	fmt.Println(encryptedData, privateKey, hashedData, owner)

	// Post the data to the postToChain

	c.String(http.StatusOK, "Data posted to chain")
}

func getBlockNumber(c *gin.Context) {
	// Get the latest block number
	blockNumber, err := client.BlockNumber(c)
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to get block number: %v", err))
		return
	}

	c.String(http.StatusOK, fmt.Sprintf("Block number: %v", blockNumber))

}
