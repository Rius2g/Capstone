package helper

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
)

func initNewNode(address string) *Node {
	return &Node{
		Address:    address,
		Neighbours: Neighbours{},
		ChordSize:  0,
	}
}

func HashToKey(hash []byte) int {
	key := int(hash[0])<<24 | int(hash[1])<<16 | int(hash[2])<<8 | int(hash[3])
	return key
}

func FillFingerTable(self *Node) *Node {
	selfId := self.Id
	//fill the finger table
	for i := 0; i < len(self.FingerTable); i++ {
		//Node nr according to formula
		add := int(math.Pow(2, float64(i)))
		key := (selfId + add) % self.ChordSize
		FT := FingerTableEntry{
			Key:         key,
			NodeId:      selfId,
			NodeAddress: self.Address}
		self.FingerTable[i] = FT
	}
	return self
}

/*
* Support function to handle errors in general
 */
func HandleError(err error, message string, fatal bool) bool {
	if err != nil {
		if fatal {
			log.Fatalf(message, "\n Errormsg: %s", err)

		} else {
			fmt.Printf(message, "\n Errormsg: %s", err)

		}
		return true
	}
	return false
}

/*
* Simple function to handle put requests.
* @returns: Returns the response as a string.
* @returns: Returns an error if it occurs.
 */
func PutRequest(url string, body string) (*http.Response, error) {

	// Create a new PUT request with the string as the request body
	req, err := http.NewRequest("PUT", url, bytes.NewBufferString(body))
	if err != nil {
		log.Printf("Failed to create PUT request: %s\n", err)
		return nil, err
	}

	// Set the appropriate Content-Type header
	req.Header.Set("Content-Type", "text/plain")

	// Send the request using a http.Client
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to send PUT request: %s\n", err)
		return nil, err
	}

	return resp, nil
}

func ClosestNodeInFingerTable(key int, node *Node) int {

	closest := -1
	distance := node.ChordSize
	//Check if key is between me and previous
	if node.Neighbours.Predecessor.Id == -1 {
		return closest
	}
	if node.Id == node.Neighbours.Predecessor.Id {
		return closest
	}
	if node.Id > node.Neighbours.Predecessor.Id {
		//if it is between me and previous then return that it is at current node
		if key <= node.Id && key > node.Neighbours.Predecessor.Id {
			return closest
		}
		//If previous is larger than current.
	} else {
		if (key > node.Neighbours.Predecessor.Id && key <= node.ChordSize) || (key >= 0 && key <= node.Id) {
			return closest
		}
	}
	//check if key is between me and next
	if node.Id < node.FingerTable[0].NodeId {
		//if it is between me and next node then return the position of next node
		if key > node.Id && key <= node.FingerTable[0].NodeId {
			return 0
		}
	} else {
		//If the next node id is smaller than current, check if the key is between current and end of chord,
		//or if the key is between 0 and the next nodes value.
		if (key > node.Id && key <= node.ChordSize) || (key >= 0 && key <= node.FingerTable[0].NodeId) {
			return 0
		}
	}
	for i := 0; i < len(node.FingerTable); i++ { //check if this still works with -1 because of abs values
		if node.FingerTable[i].NodeId != -1 {
			newDistance := CalcDistance(node.ChordSize, node.FingerTable[i].NodeId, key)
			if Abs(newDistance) < distance {
				closest = i
				distance = Abs(newDistance)
			}
		}
	}
	return closest
}

/*
* Support function to handle the distance calculation.
* @param chordsize is the size of the chord.
* @param nodeID is the id of the node that is being compared to.
* @param key is the key that is being compared to the node.
 */
func CalcDistance(chordsize int, nodeID int, key int) int {
	distance := (key - nodeID + chordsize) % chordsize

	return distance
}

/*
* Support function to handle the abs function.
* Used in distance calculation.
 */
func Abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func FindNextToPreviousInFingerTable(self *Node, SuccsessorInt int) int { //this might be good??
	closest := -1
	idJump := 0
	for i := 0; i < len(self.FingerTable); i++ {
		if self.FingerTable[i].NodeId < SuccsessorInt && self.FingerTable[i].NodeId > idJump && self.FingerTable[i].NodeId != -1 && self.FingerTable[i].NodeId != self.Id {
			closest = i
			idJump = self.FingerTable[i].NodeId
		}
	}
	if closest == -1 { //did not find a entry that was smaller than successor
		closest = FindNonEmpty(self, SuccsessorInt)
		if closest == -1 {
			log.Fatalf("All entries in fingertable is either unset or crashed node.\n")
		}
	}

	return closest
}

func FindNonEmpty(self *Node, SuccsessorInt int) int {
	index := -1
	for i := (len(self.FingerTable) - 1); i <= 0; i-- {
		if self.FingerTable[i].NodeId != -1 && self.FingerTable[i].NodeId != self.Id && self.FingerTable[i].NodeId != SuccsessorInt {
			return i
		}
	}
	return index
}

/*
* Simple function to handle get requests.
* @param url is the url that is being requested.
* @returns: Returns the response as a string.
* @returns: Returns an error if it occurs.
 */
func GetRequest(url string) (string, error, int) { //returns the response as an int
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("Error on get request\n Errormsg: %s", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error on reading body. \n Errormsg: %s", err)
		return "", err, resp.StatusCode
	}

	response := string(body)

	return response, nil, resp.StatusCode

}

func CreateFingerTable(chordSize int) []FingerTableEntry {
	//set the finger table entries in the finger table
	finger_table_size := int(math.Log2(float64(chordSize))) //this is the size of the finger table
	ft := make([]FingerTableEntry, finger_table_size)
	return ft
}

/*
* Simple function to handle post requests.
 */
func PostRequest(url string) (string, error) {
	resp, err := http.Post(url, "text/plain", nil)
	if err != nil {
		log.Printf("Error on post request. \n Errormsg: %s", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error on reading body. \n Errormsg: %s", err)
	}

	response := string(body)

	return response, err
}
