package main

import (
    "context"
    "crypto/sha256"
    "encoding/json"
    "fmt"
    "log"
    "os"
    "math/big"
    "strconv"
    "strings"
    "time"
    h "web3server/helper"
    "github.com/ethereum/go-ethereum/common/hexutil"
    "github.com/ethereum/go-ethereum"
    "github.com/ethereum/go-ethereum/accounts/abi"
    "github.com/ethereum/go-ethereum/accounts/abi/bind"
    "github.com/ethereum/go-ethereum/crypto"
    "github.com/ethereum/go-ethereum/ethclient"
    "github.com/ethereum/go-ethereum/core/types"
    "github.com/ethereum/go-ethereum/common"
    "github.com/gin-gonic/gin"
    "github.com/joho/godotenv"
)

type Contract struct {
    ABI json.RawMessage `json:"abi"`
}

func LoadABI() (abi.ABI, error) {
    filePath := "TwoPhaseCommit.json"
    abiBytes, err := os.ReadFile(filePath)
    if err != nil {
        return abi.ABI{}, fmt.Errorf("failed to read ABI file: %v", err)
    }

    var contract Contract
    err = json.Unmarshal(abiBytes, &contract)
    if err != nil {
        return abi.ABI{}, fmt.Errorf("failed to unmarshal contract JSON: %v", err)
    }

    parsedABI, err := abi.JSON(strings.NewReader(string(contract.ABI)))
    if err != nil {
        return abi.ABI{}, fmt.Errorf("failed to parse ABI JSON: %v", err)
    }

    return parsedABI, nil
}

var client *ethclient.Client
var contractAddress common.Address
var contractABI abi.ABI
var PrivateKey string

func main() {
    if err := godotenv.Load(); err != nil {
        log.Fatalf("Error loading .env file")
    }
    PrivateKey = "0x" + MustGetEnv("PRIVATE_KEY")

    var err error
    client, err = ethclient.Dial("wss://api.avax-test.network/ext/bc/C/ws")
    if err != nil {
        log.Fatalf("Failed to connect to the Ethereum client: %v", err)
    }
    defer client.Close()

    blockNumber, err := client.BlockNumber(context.Background())
    if err != nil {
        log.Fatalf("Failed to get the latest block number: %v", err)
    }
    fmt.Printf("Latest block number: %d\n", blockNumber)

    contractAddress = common.HexToAddress("0xA7F043397395F9d16782E6FC5c502AB5643078b3")
    contractABI, err = LoadABI()
    if err != nil {
        log.Fatalf("Failed to parse contract ABI: %v", err)
    }

    go Web3Listener()

    router := gin.Default()
    router.POST("/upload", postData)
    router.GET("/get/:dataname/:owner", getData)

    fmt.Println("Server is running on port 8080")
    router.Run(":8080")
}

func postData(c *gin.Context) {
    data := c.PostForm("data")
    owner := c.PostForm("owner")
    dataName := c.PostForm("dataname")
    releaseTime := c.PostForm("releaseTime")

    encryptedData, privKey, err := h.EncryptData(data)
    if err != nil {
        c.JSON(400, gin.H{"error": fmt.Sprintf("Failed to encrypt data: %v", err)})
        return
    }

    ReleaseTime, err := strconv.ParseUint(releaseTime, 10, 64)
    if err != nil {
        c.JSON(400, gin.H{"error": fmt.Sprintf("Failed to convert release time to uint64: %v", err)})
        return
    }

    hash := sha256.Sum256([]byte(data + owner))

    privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(PrivateKey, "0x"))
    if err != nil {
        c.JSON(500, gin.H{"error": fmt.Sprintf("Failed to parse private key: %v", err)})
        return
    }

    auth, err := bind.NewKeyedTransactorWithChainID(privateKey, big.NewInt(43113))
    if err != nil {
        c.JSON(500, gin.H{"error": fmt.Sprintf("Failed to create authenticated transactor: %v", err)})
        return
    }

    input, err := contractABI.Pack("addStoredData", encryptedData, privKey, owner, dataName, big.NewInt(int64(ReleaseTime)), hash[:])
    if err != nil {
        c.JSON(500, gin.H{"error": fmt.Sprintf("Failed to pack transaction data: %v", err)})
        return
    }

    gasPrice, err := client.SuggestGasPrice(context.Background())
    if err != nil {
        c.JSON(500, gin.H{"error": fmt.Sprintf("Failed to suggest gas price: %v", err)})
        return
    }

    gasLimit, err := client.EstimateGas(context.Background(), ethereum.CallMsg{
        From:     auth.From,
        To:       &contractAddress,
        Gas:      0,
        GasPrice: gasPrice,
        Value:    big.NewInt(0),
        Data:     input,
    })
    if err != nil {
        c.JSON(500, gin.H{"error": fmt.Sprintf("Failed to estimate gas limit: %v", err)})
        return
    }

    gasLimit = uint64(float64(gasLimit) * 1.1)

    nonce, err := client.PendingNonceAt(context.Background(), auth.From)
    if err != nil {
        c.JSON(500, gin.H{"error": fmt.Sprintf("Failed to retrieve account nonce: %v", err)})
        return
    }

    tx := types.NewTransaction(nonce, contractAddress, big.NewInt(0), gasLimit, gasPrice, input)

    signedTx, err := types.SignTx(tx, types.NewEIP155Signer(big.NewInt(43113)), privateKey)
    if err != nil {
        c.JSON(500, gin.H{"error": fmt.Sprintf("Failed to sign transaction: %v", err)})
        return
    }

    err = client.SendTransaction(context.Background(), signedTx)
    if err != nil {
        c.JSON(500, gin.H{"error": fmt.Sprintf("Failed to send transaction: %v", err)})
        return
    }

    receipt, err := bind.WaitMined(context.Background(), client, signedTx)
    if err != nil {
        c.JSON(500, gin.H{"error": fmt.Sprintf("Failed to get transaction receipt: %v", err)})
        return
    }

    c.JSON(200, gin.H{
        "message":         "Data published successfully",
        "transactionHash": receipt.TxHash.Hex(),
        "blockNumber":     receipt.BlockNumber.Uint64(),
    })
}

func getData(c *gin.Context) {
    dataName := c.Param("dataname")
    owner := c.Param("owner")

    input, err := contractABI.Pack("GetPublicData", dataName, owner)
    if err != nil {
        c.JSON(500, gin.H{"error": fmt.Sprintf("Failed to pack transaction data: %v", err)})
        return
    }

    msg := ethereum.CallMsg{
        To:   &contractAddress,
        Data: input,
    }

    output, err := client.CallContract(context.Background(), msg, nil)
    if err != nil {
        c.JSON(500, gin.H{"error": fmt.Sprintf("Failed to call contract: %v", err)})
        return
    }

    var result struct {
        EncryptedData []byte
        Hash          [32]byte
        Owner         string
        DataName      string
        ReleaseTime   *big.Int
    }

    tupleBytes := output

    encryptedDataOffset := new(big.Int).SetBytes(tupleBytes[:32]).Uint64()
    encryptedDataLength := new(big.Int).SetBytes(tupleBytes[encryptedDataOffset:encryptedDataOffset+32]).Uint64()
    result.EncryptedData = tupleBytes[encryptedDataOffset+32 : encryptedDataOffset+32+encryptedDataLength]

    copy(result.Hash[:], tupleBytes[32:64])

    ownerOffset := new(big.Int).SetBytes(tupleBytes[64:96]).Uint64()
    ownerLength := new(big.Int).SetBytes(tupleBytes[ownerOffset:ownerOffset+32]).Uint64()
    result.Owner = string(tupleBytes[ownerOffset+32 : ownerOffset+32+ownerLength])

    dataNameOffset := new(big.Int).SetBytes(tupleBytes[96:128]).Uint64()
    dataNameLength := new(big.Int).SetBytes(tupleBytes[dataNameOffset:dataNameOffset+32]).Uint64()
    result.DataName = string(tupleBytes[dataNameOffset+32 : dataNameOffset+32+dataNameLength])

    result.ReleaseTime = new(big.Int).SetBytes(tupleBytes[128:160])

    fmt.Printf("Manually decoded result: %+v\n", result)

    response := gin.H{
        "encryptedData": hexutil.Encode(result.EncryptedData),
        "hash":          hexutil.Encode(result.Hash[:]),
        "owner":         result.Owner,
        "dataName":      result.DataName,
        "releaseTime":   result.ReleaseTime.String(),
    }

    c.JSON(200, response)
}

func Web3Listener() {
    query := ethereum.FilterQuery{
        Addresses: []common.Address{contractAddress},
    }

    logs := make(chan types.Log)
    sub, err := client.SubscribeFilterLogs(context.Background(), query, logs)
    if err != nil {
        log.Fatalf("Failed to subscribe to logs: %v", err)
    }
    defer sub.Unsubscribe()

    for {
        select {
        case err := <-sub.Err():
            log.Printf("Subscription error: %v", err)
            time.Sleep(5 * time.Second)
            sub, err = client.SubscribeFilterLogs(context.Background(), query, logs)
            if err != nil {
                log.Printf("Failed to resubscribe to logs: %v", err)
            }
        case vLog := <-logs:
            handleLog(vLog)
        }
    }
}

func handleLog(vLog types.Log) {
    switch vLog.Topics[0].Hex() {
    case contractABI.Events["PushEncryptedData"].ID.Hex():
        HandlePushEncryptedDataEvent(vLog)
    case contractABI.Events["PushPrivateKey"].ID.Hex():
        HandlePushPrivateKeyEvent(vLog)
    default:
        log.Println("Unknown event:", vLog.Topics[0].Hex())
    }
}

func HandlePushEncryptedDataEvent(vLog types.Log) {
    var Event h.PushEncrytedDataEvent

    err := contractABI.UnpackIntoInterface(&Event, "PushEncryptedData", vLog.Data)
    if err != nil {
        log.Fatalf("Failed to unpack event: %v", err)
    }

    fmt.Println("Encrypted Data: ", Event.EncryptedData)
    fmt.Println("Owner: ", Event.Owner)
    fmt.Println("Data Name: ", Event.DataName)
    fmt.Println("Hash: ", Event.Hash)
}

func HandlePushPrivateKeyEvent(vLog types.Log) {
    var Event h.PushPrivateKeyEvent

    err := contractABI.UnpackIntoInterface(&Event, "PushPrivateKey", vLog.Data)
    if err != nil {
        log.Fatalf("Failed to unpack event: %v", err)
    }

    fmt.Println("Decryption Key: ", Event.DecryptionKey)
    fmt.Println("Owner: ", Event.Owner)
    fmt.Println("Data Name: ", Event.DataName)
    fmt.Println("Hash: ", Event.Hash)
}

func MustGetEnv(key string) string {
    value := os.Getenv(key)
    if value == "" {
        log.Fatalf("Environment variable %s must be set", key)
    }
    return value
}
