package main

import (
	chord "Backend/chord"
	h "Backend/helper"
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
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

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
* Function to handle the /neighbors GET request
* @param c is the context of the get request.
* @returns: Responds to the request
 */
func GetNeighborHandler(c *gin.Context) {
	neighbors := chord.FindNeighbors(self)
	//response := h.VerifyNeighbors{Self: self.Id, Neighbours: neighbors}
	c.JSON(http.StatusOK, neighbors)
}

/*
* Return the chord size to the new node in the chord for it to hash with
 */
func getInfo(c *gin.Context) {
	log.Printf("sending info from %s\n", self.Address)

	log.Printf("sending info from %d\n", self.Id)
	c.String(http.StatusOK, strconv.Itoa(self.ChordSize))
}


func FindClosestSeedToReceiveFrom(c *gin.Context){
//this function shall be called by non-seeds to find the closest seed in the network in terms of RTT to receive the data from
}

/*
// Function to handle a simulated node crash
// @param c is the context of the get request.
// @returns: Responds to the request
*/
func NodeCrash(c *gin.Context) { //crash the node
	c.String(http.StatusOK, "Node %d Crashed. My Chordsize was: %d\n", self.Id, self.ChordSize)
}

/*
* Function to balance the chord network again
* @param c is the context of the get request.
* @returns: Responds to the request
 */
func BalanceChord(c *gin.Context) {
	//read nodeLoc body, if same id as us, we stop as we are around the circle
	//if not us, we check finder table if we are more appropriate value in it and send forward.
	//more appropriate if this node id is >= the key but < the current entry
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.String(http.StatusConflict, "Cant read body here :(")
		return
	}

	var updatedNode h.Node
	if err := json.Unmarshal(body, &updatedNode); err != nil {
		c.String(http.StatusConflict, "Can't parse JSON body :(")
		return
	}
	if updatedNode.Id != self.Id {
		go chord.UpdateFingertableOnJoin(updatedNode, self)
	} else {
		c.String(http.StatusOK, "")
		return
	}
}

/*
* Function to handle a node recover
* @param c is the context of the get request.
* @returns: Responds to the request
 */
func NodeRecover(c *gin.Context) {

	if self.ChordSize == 0 { //this means not in chord anymore we should request to rejoin
		//we send a rejoin request
		url := "http://" + self.FingerTable[0].NodeAddress + "/rejoin-request"
		body := strings.NewReader(self.Address)
		_, err := http.Post(url, "text/plain", body) //send our address to the node we want to request our rejoin
		if err != nil {
			h.HandleError(err, "Error on post request.", false)
		}

	}
	c.String(200, "OK")

}

func RejoinChord(c *gin.Context) {
	//we send join to the node that sent us this request with our ip:port
	body := c.Request.Body
	newNode, err := io.ReadAll(body)
	url := "http://" + string(newNode) + "/join?nprime=" + self.Address
	_, err = http.Post(url, "text/plain", nil)
	if err != nil {
		h.HandleError(err, "Error on post request.", false)
	}
	c.String(200, "OK")

}

/*
* Endpoint to handle finding appropriate node for a key in the chord
 */
func FindNodeId(c *gin.Context) {
	nodeId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		h.HandleError(err, "Could not convert id\n", false)
	}

	resultChan := make(chan h.Node)
	defer close(resultChan)

	go chord.FindNodeId(nodeId, self, resultChan)
	value := <-resultChan
	c.JSON(http.StatusOK, value)
}

/*
* This API is called after a node has joined the network.
* This is receive by the succsessor of the new node.
* Receive its previous node and update its own previous node.
 */
func UpdatePreviousOnsuccsessor(c *gin.Context) {
	//we send the nodes h.Nodeloc in here
	//we send this nodes H.nodeToloc back and update to new previous
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.HandleError(err, "Can't read body", false)
	}

	var previous h.Node
	err = json.Unmarshal(body, &previous)
	if err != nil {
		h.HandleError(err, "Can't unmarshal body", false)
	}

	prev := self.Neighbours.Predecessor
	if prev.Id == -1 { //empty entry we add ourselves to send
		prev.Id = self.Id
		prev.Address = self.Address
	}
	self.Neighbours.Predecessor = &previous

	c.JSON(http.StatusOK, prev)
}

/*
* This API is called after removing all nodes have removed the leaving node from their finger tables.
* This will fill finger table and send the api forward in the chord
* When the api reaches the node that started it, it will exit.
 */
func FillFingerTable(c *gin.Context) { //done filling finger table
	//need a api to call when we want to fill empty entries in the finger table
	//needs a way to know when we are done filling the table and should exit
	id, err := strconv.Atoi(c.Param(("id")))
	if err != nil {
		h.HandleError(err, "Could not convert id\n", false)
	}

	go chord.FillEmptyEntries(self, id)

	c.String(http.StatusOK, "Filled FingerTables")
}

func GetIdOnIp(c *gin.Context) {

	c.String(http.StatusOK, strconv.Itoa(self.Id))
}

func EvictGossip(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if h.HandleError(err, "Can't read body", false) {
		return
	}
	// Split the body into two strings.
	bodyStrings := strings.Split(string(body), " ")
	startAddress := bodyStrings[0]
	startId, err := strconv.Atoi(bodyStrings[1])
	if h.HandleError(err, "Can't convert id to int", false) {
		return
	}

	id, err := strconv.Atoi(c.Param(("id"))) //node to remove
	if err != nil {
		h.HandleError(err, "Could not convert id\n", false)
	}
	chord.EvictGossip(id, startId, startAddress, self)
	c.String(http.StatusOK, "Got Evict")
}

func FindPrevious(c *gin.Context) {
	//finds the previous ndoe of a node that has crashed and shall be evicted

	id, err := strconv.Atoi(c.Param(("id"))) //node of crashed, we want to find node that has this as succsessor
	if h.HandleError(err, "Can't convert id to int", false) {
		return
	}

	go chord.FindPrevious(id, self)
	c.String(http.StatusOK, "Got FindPrevious")
}

func PingNeighbors() { //ping succsessor (if it exists) and predecessor
	successor := self.FingerTable[0].NodeAddress
	predecessor := self.Neighbours.Predecessor.Address
	if successor != "" {
		url := "http://" + successor + "/ping-node"
		_, err, resp := h.GetRequest(url)
		if err != nil {
			h.HandleError(err, "Error on post request.", false)
		}
		if resp == http.StatusInternalServerError {
			go chord.FindPrevious(self.FingerTable[0].NodeId, self)
		}
	}

	if predecessor != "" {
		url := "http://" + predecessor + "/ping-node"
		_, err, resp := h.GetRequest(url)
		if err != nil {
			h.HandleError(err, "Error on post request.", false)
		}
		if resp == http.StatusInternalServerError {
			go chord.FindPrevious(self.Neighbours.Predecessor.Id, self)
		}
	}

	time.Sleep(4 * time.Second)
}


func AttatchNodeToSeed() { //called to attach this node to a seed that is close 
    go chord.FindClosestSeed(self, "") 
}


func GetNodeAttatched(c *gin.Context) {
    nodeAddress := c.Param("nodeAddress")
    RTT, err := strconv.Atoi(c.Param("RTT"))
    if err != nil {
        log.Println("Error converting RTT to int")
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Error converting RTT to int"})
        return
    }

    nodeEntry := h.RTTEntry{Address: nodeAddress, RTTTime: uint64(RTT)}
    go chord.AttatchNodeToSeed(self, nodeEntry)

    c.String(http.StatusOK, "Node attatched to seed")
}

func RemoveNodeFromSeed(c *gin.Context) {
    address := c.Param("adress")
    go chord.FindClosestSeed(self, address)
    c.String(http.StatusOK, "Node removed from seed and found new one")
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
	r.GET("/RTT", Ping)
	r.GET("/getFromChain", getFromChain)
	r.GET("/node-id/:id", FindNodeId)
	r.POST("/start", StartChordHandler)
	r.GET("/storage/neighbors", GetNeighborHandler)
	r.GET("/node-id/:id", FindNodeId)
	r.GET("/fill-fingertable/:id", FillFingerTable)
	r.GET("/getInfo", getInfo)
	r.GET("/find-previous/:id", FindPrevious)
    r.GET("/RemoveNodeFromSeed:/adress", RemoveNodeFromSeed)

	r.PUT("/update-previous-on-succsessor", UpdatePreviousOnsuccsessor)
	r.PUT("/gossip", BalanceChord)

	r.PUT("/evict-gossip/:id", EvictGossip)

	r.POST("/leave", NodeLeave)
	r.POST("/join", NodeJoin)
	r.POST("/sim-crash", NodeCrash)
	r.POST("/sim-recover", NodeRecover)
	r.POST("/start", StartChordHandler)
	r.POST("/rejoin-request", RejoinChord)

	r.POST("/postToChain", postToChain)

	// Run the r
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
