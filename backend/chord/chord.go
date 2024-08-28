package chord

import (
	h "Backend/helper"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"
)

func returnNeighbours(self *h.Node) (h.Node, h.Node) {
	// Returns the predecessor and successor of the node
	return *self.Neighbours.Predecessor, *self.Neighbours.Successor
}

var wg sync.WaitGroup

func CreateChord(self *h.Node) *h.Node {
	Address_hash := sha1.Sum([]byte(self.Address))
	self.Id = (h.HashToKey(Address_hash[:]) % self.ChordSize)
	self.Neighbours.Predecessor = self
	self.Neighbours.Successor = self
	self.FingerTable = CreateFingerTable(self.ChordSize)
	self = h.FillFingerTable(self)
	return self
}



/*
* Finds the neighbours of the current node.
* @param self is the current node.
* @returns: Returns the neighbors as json.
 */
func FindNeighbors(self *h.Node) h.Neighbours { //returns the previous and next nodes of the current node in the chord ring

	return self.Neighbours
}

/*
* Creates a fully initialized node
* @param selfId is the Id of the current node.
* @param nodeAmount is the total amount of nodes.
* @param nodes is a list of all other nodes and their location.
* @param chordSize is the size of the chord-ring.
* @returns: returns a fully initialized node.
 */
func InitNodeNew(selfAddress string) h.Node {
	var FingerTable []h.FingerTableEntry
	node := h.Node{FingerTable: FingerTable, Id: 0, Address: selfAddress, ChordSize: 0}
	node.Neighbours = h.Neighbours{Predecessor: &node, Successor: &node}
    node.SeedToReceiveFrom = h.RTTEntry{} 
    node.Seeds = []string{}
    node.Seed = false
    node.NodesToSendTo = []h.RTTEntry{}
	return node
}

/*
* Updates a node when it joins a chord ring
* @param self is the current node.
 */
func UpdateNode(self *h.Node) h.Node {
	Address_hash := sha1.Sum([]byte(self.Address))
	self.Id = (h.HashToKey(Address_hash[:]) % self.ChordSize)
	self.FingerTable = CreateFingerTable(self.ChordSize)
    self = h.FillFingerTable(self)

	return *self
}

/*
* creates an empty fingertable with a static size in the node..
* @param nodeAmount is the total number of nodes.
* @Returns: empty fingertable
 */
func CreateFingerTable(chordSize int) []h.FingerTableEntry {
	//set the finger table entries in the finger table
	finger_table_size := int(math.Log2(float64(chordSize))) //this is the size of the finger table
	FingerTable := make([]h.FingerTableEntry, finger_table_size)
	return FingerTable
}

/*
* We forward our KVPs to succsessor
* We forward previous to succsessor
* Succsessor starts gossip
* @param id is the id of the node that leFingerTable
* @param self is the current node.
 */
func NodeLeFingerTable(self *h.Node) {
	url := "http://" + self.FingerTable[0].NodeAddress + "/kvp-leFingerTable"
	//need to add the keys to a body

	go func() {
		_, err, statusCode := h.GetRequest(url) //send body as put
		if statusCode != http.StatusOK {
			wg.Add(1)
			go func() {
				FindPrevious(self.FingerTable[0].NodeId, self)
				wg.Done()
			}()
			wg.Wait()
			NodeLeFingerTable(self)
			return
		}
		if err != nil {
			h.HandleError(err, "Error on KVP transfer on leFingerTable", false)
			return
		}
	}()
	go func() {
		FillUrl := "http://" + self.Neighbours.Predecessor.Address + "/find-previous/" + strconv.Itoa(self.Id)
		_, err, statusCode := h.GetRequest(FillUrl)
		if statusCode == http.StatusInternalServerError {
			wg.Add(1)
			go func() {
				FindPrevious(self.FingerTable[0].NodeId, self)
				wg.Done()
			}()
			wg.Wait()
			NodeLeFingerTable(self)
			return
		}
		if err != nil {
			h.HandleError(err, "Error on FindPrevious", false)
		}
		wg.Wait()
		self.ChordSize = 0
		self.FingerTable = nil
		self.Neighbours = h.Neighbours{}
	}()
}

func EvictGossip(evictId int, senderid int, senderIp string, self *h.Node) { //remove the node from the finger table
	for i := 0; i < len(self.FingerTable); i++ { //set empty entries
		if self.FingerTable[i].NodeId == evictId {
			self.FingerTable[i].NodeId = -1
			self.FingerTable[i].NodeAddress = ""
		}
	}
	if self.Neighbours.Predecessor.Id == evictId { //update the previous entry to replace the evicted node
		wg.Add(1)
		self.Neighbours.Predecessor.Id = senderid
		self.Neighbours.Predecessor.Address = senderIp
		go func() {
			FillUrl := "http://" + self.Neighbours.Predecessor.Address + "/fill-fingertable/" + strconv.Itoa(self.Id) //url of the request
			_, err, statusCode := h.GetRequest(FillUrl)
			if err != nil {
				h.HandleError(err, "Error on fill finger table start", false)
				return
			}
			if statusCode == http.StatusInternalServerError {
				wg.Add(1)
				go func() {
					FindPrevious(self.Neighbours.Predecessor.Id, self)
					wg.Done()
				}()
				wg.Wait()
				EvictGossip(evictId, senderid, senderIp, self)
			}
			wg.Done()
		}()
		wg.Wait()
		return
	} else {
		wg.Add(1)
		go func() {
			url := "http://" + self.Neighbours.Predecessor.Address + "/evict-gossip/" + strconv.Itoa(evictId) //url of the request
			body := senderIp + " " + strconv.Itoa(senderid)
			go func() {
				resp, err := h.PutRequest(url, body) //send body as put
				if err != nil {
					h.HandleError(err, "Error on gossip leave forwarding", false)
					return
				}
				if resp.StatusCode == http.StatusInternalServerError {
					wg.Add(1)
					go func() {
						FindPrevious(self.Neighbours.Predecessor.Id, self)
						wg.Done()
					}()
					wg.Wait()
					EvictGossip(evictId, senderid, senderIp, self)
					return
				}
			}()
			wg.Done()
		}()
		wg.Wait()
	}
}

// this is the other entry point if node is not previous in first case
func FindPrevious(evictionId int, self *h.Node) { //finds previous and starts the gossip chain to evict the node¨
	if self.FingerTable[0].NodeId == evictionId {
		for i := 0; i < len(self.FingerTable); i++ {
			if self.FingerTable[i].NodeId == evictionId {
				self.FingerTable[i].NodeId = -1
				self.FingerTable[i].NodeAddress = ""
			}
		}
		wg.Add(1)
		evicturl := "http://" + self.Neighbours.Predecessor.Address + "/evict-gossip/" + strconv.Itoa(evictionId) //url of the request
		body := self.Address + " " + strconv.Itoa(self.Id)
		go func() {
			resp, err := h.PutRequest(evicturl, body) //send body as put
			if err != nil {
				h.HandleError(err, "Error on evict gossip", false)
			}
			if resp.StatusCode == http.StatusInternalServerError {
				wg.Add(1)
				go func() {
					FindPrevious(self.Neighbours.Predecessor.Id, self)
					wg.Done()
				}()
				wg.Wait()
				FindPrevious(evictionId, self)
				return
			}
			wg.Done()
		}()
		wg.Wait()
	} else {
		go func() {
			FingerTable_id := h.FindNextToPreviousInFingerTable(self, evictionId)
			FindPreviousurl := "http://" + self.FingerTable[FingerTable_id].NodeAddress + "/find-previous/" + strconv.Itoa(evictionId) //url of the request
			wg.Add(1)
			go func() {
				_, err, statusCode := h.GetRequest(FindPreviousurl)
				if err != nil {
					h.HandleError(err, "Error on FindPrevious", false)
				}
				wg.Done()
				if statusCode == http.StatusInternalServerError {
					wg.Add(1)
					go func() {
						FindPrevious(self.FingerTable[FingerTable_id].NodeId, self)
						wg.Done()
					}()
					wg.Wait()
					FindPrevious(evictionId, self)
					return
				}
			}()
			wg.Wait()
		}()
	}
}

/*
* We forward our KVPs to succsessor
* We forward previous to succsessor
* Succsessor starts gossip
* @param id is the id of the node that left
* @param self is the current node.
 */
func NodeLeft(self *h.Node) {
	url := "http://" + self.FingerTable[0].NodeAddress + "/kvp-left"
	//need to add the keys to a body

	go func() {
		_, err, statusCode := h.GetRequest(url) //send body as put
		if statusCode != 200 {
			wg.Add(1)
			go func() {
				FindPrevious(self.FingerTable[0].NodeId, self)
				wg.Done()
			}()
			wg.Wait()
			NodeLeft(self)
			return
		}
		if err != nil {
			h.HandleError(err, "Error on KVP transfer on left", false)
			return
		}
	}()
	go func() {
		FillUrl := "http://" + self.Neighbours.Predecessor.Address + "/find-previous/" + strconv.Itoa(self.Id)
		_, err, statusCode := h.GetRequest(FillUrl)
		if statusCode == http.StatusInternalServerError {
			wg.Add(1)
			go func() {
				FindPrevious(self.FingerTable[0].NodeId, self)
				wg.Done()
			}()
			wg.Wait()
			NodeLeft(self)
			return
		}
		if err != nil {
			h.HandleError(err, "Error on FindPrevious", false)
		}
		wg.Wait()
		self.ChordSize = 0
		self.FingerTable = nil
		self.Neighbours = h.Neighbours{}
	}()
}

/*
* Function to handle node joining the chord ring
* This is for the node that IS joining the chord
* @param self is the current node (that is joining)
* @param ip_port is the ip and port of the node that is already in the chord ring (temp succsessor)
 */
func NodeJoin(self *h.Node, HostPort string) {
	getInfo := "http://" + HostPort + "/getInfo"
	chordSize, err, _ := h.GetRequest(getInfo)
	if err != nil {
		log.Printf("Error on leave request")
	}
	//hash with chordsize
	self.ChordSize, err = strconv.Atoi(chordSize)
	if h.HandleError(err, "Could not convert chord size to int", false) {
		return
	}

	*self = UpdateNode(self)

	Address_hash := sha1.Sum([]byte(HostPort))
	nodeId := (h.HashToKey(Address_hash[:]) % self.ChordSize)

	//then we add succsessor to first index
	self.FingerTable[0] = h.FingerTableEntry{NodeId: nodeId, NodeAddress: HostPort, Key: self.Id + 1}

	DynamicFingerTableFill(self)
	wg.Add(1)
	gossip := "http://" + self.FingerTable[0].NodeAddress + "/gossip" //start the gossip chain
	type SelfInfo struct {
		Id        int
		Ipaddress string
	}

	selfInfo := SelfInfo{Id: self.Id, Ipaddress: self.Address}

	requestBody, _ := json.Marshal(selfInfo)
	resp, err := h.PutRequest(gossip, string(requestBody))
	wg.Done()
	if resp.StatusCode == http.StatusInternalServerError {
		wg.Add(1)
		go func() {
			FindPrevious(self.FingerTable[0].NodeId, self)
			wg.Done()
		}()
		wg.Wait()
		// NodeJoin(self, HostPort)
		return
	}
	if err != nil {
		h.HandleError(err, "Could not start gossip", false)

	}
	wg.Add(1)
	updatePrevious := "http://" + self.FingerTable[0].NodeAddress + "/update-previous-on-succsessor" //url of the request
	body, err := h.PutRequest(updatePrevious, string(requestBody))
	wg.Done()
	if body.StatusCode == http.StatusInternalServerError {
		wg.Add(1)
		go func() {
			FindPrevious(self.FingerTable[0].NodeId, self)
			wg.Done()
		}()
		wg.Wait()
		NodeJoin(self, HostPort)
		return
	}
	if err != nil {
		h.HandleError(err, "Could not update succsessors previous", false)
	}

	bodyBytes, err := io.ReadAll(body.Body)
	if err != nil {
		h.HandleError(err, "Could not read body", false)
	}

	var previousNode h.Node
	err = json.Unmarshal(bodyBytes, &previousNode)
	if err != nil {
		h.HandleError(err, "Could not read body", false)
	}
	wg.Wait()
	self.Neighbours.Predecessor = &previousNode
}

/*
* This is the gossip function
* Helper function to handle node joining the chord ring
* Received by all nodes in the chord to update the finger table in regards to new entry (if better fit than a entry)
* @param node is the node that is joining
* @param self is the current node which the request is forwarded to
 */
func UpdateFingertableOnJoin(node h.Node, self *h.Node) {
	n := node.Id
	for i := 0; i < len(self.FingerTable); i++ {
		p := self.FingerTable[i].NodeId
		k := self.FingerTable[i].Key
		if ((n < p) && (p < k)) ||
			((k <= n) && (n < p)) ||
			((0 <= p) && (p < k) && (k <= n) && (n < self.ChordSize)) {
			self.FingerTable[i].NodeId = node.Id
			self.FingerTable[i].NodeAddress = node.Address
		}
	}

	//forward to successor
	gossip := "http://" + self.FingerTable[0].NodeAddress + "/gossip" //url of the request

	requestBody, _ := json.Marshal(node)

	resp, err := h.PutRequest(gossip, string(requestBody)) //send body as put
	if resp.StatusCode == http.StatusInternalServerError {
		wg.Add(1)
		go func() {
			FindPrevious(self.FingerTable[0].NodeId, self)
			wg.Done()
		}()
		wg.Wait()
		// UpdateFingertableOnJoin(node, self)
		return
	}
	if err != nil {
		h.HandleError(err, "Error on gossip", false)
	}
}

/*
* Called by joining node to fill finger table
* Goes over the finger table and fills it with the appropriate nodes by asking the chord ring for best fits
* @param self is the current node (that is joining)
 */
func DynamicFingerTableFill(self *h.Node) { //this is new node so it only knows about one other node in the chord ring
	for i := 0; i < len(self.FingerTable); i++ {
		key := (self.Id + int(math.Pow(2, float64(i)))) % self.ChordSize //this is the id we are searching for
		//we need to find the node that is closest to the id we are searching for
		//we send a request to the node that is closest to the id we are searching for
		//we then update our finger table with the node we found

		url := "http://" + self.FingerTable[0].NodeAddress + "/node-id/" + strconv.Itoa(key) //url of the request
		response, err, statusCode := h.GetRequest(url)
		if statusCode == http.StatusInternalServerError {
			wg.Add(1)
			go func() {
				FindPrevious(self.FingerTable[0].NodeId, self)
				wg.Done()
			}()
			wg.Wait()
			DynamicFingerTableFill(self)
			return
		}
		if err != nil {
			h.HandleError(err, "Error on dynamic FingerTable fill", false)
		}
		NodeAddress := h.Node{}
		json.Unmarshal([]byte(response), &NodeAddress)
		self.FingerTable[i] = h.FingerTableEntry{NodeId: NodeAddress.Id, NodeAddress: NodeAddress.Address, Key: key}
	}
}

/*
* Called by all nodes in chord when a node leaves
* Called aFingerTableer the node has been removed from the finger table
* Fills the empty entries in the finger table by asking for new keys from rest of the chord
* @param self is the current node
 */
func FillEmptyEntries(self *h.Node, senderId int) { //this is new node so it only knows about one other node in the chord ring
	index := 0
	for j := 0; j < len(self.FingerTable); j++ {
		if self.FingerTable[j].NodeId != -1 {
			index = j
			break
		}
	}
	for i := 0; i < len(self.FingerTable); i++ {
		if self.FingerTable[i].NodeId == -1 {
			key := (self.Id + int(math.Pow(2, float64(i)))) % self.ChordSize
			url := "http://" + self.FingerTable[index].NodeAddress + "/node-id/" + strconv.Itoa(key)
			response, err, statusCode := h.GetRequest(url)
			if statusCode == http.StatusInternalServerError {
				wg.Add(1)
				go func() {
					FindPrevious(self.Neighbours.Predecessor.Id, self)
					wg.Done()
				}()
				wg.Wait()
				FillEmptyEntries(self, senderId)
				return
			}
			if err != nil {
				h.HandleError(err, "Error on dynamic finger table fill", false)
			}
			node_loc := h.Node{}
			json.Unmarshal([]byte(response), &node_loc)
			self.FingerTable[i].NodeId = node_loc.Id
			self.FingerTable[i].NodeAddress = node_loc.Address
			self.FingerTable[i].Key = key
		}
	}
	if self.Id != senderId {
		go func() {
			FillUrl := "http://" + self.Neighbours.Predecessor.Address + "/fill-fingertable/" + strconv.Itoa(senderId) //url of the request
			_, err, statusCode := h.GetRequest(FillUrl)
			if err != nil {
				h.HandleError(err, "Error on fill finger table start", false)
				return
			}
			if statusCode == http.StatusInternalServerError {
				wg.Add(1)
				go func() {
					FindPrevious(self.Neighbours.Predecessor.Id, self)
					wg.Done()
				}()
				wg.Wait()
				FillEmptyEntries(self, senderId)
				return
			}
		}()
	}
}

/*
* API function to get the NodeAddress that is the best fit
* Returns the best fit for finger tables
* @param node_id is the id of the node that is being searched for
* @param self is the current node
* @param resultChan is the channel where we send the response to have threading
 */
func FindNodeId(node_id int, self *h.Node, resultChan chan h.Node) { //used to find a key in the chord ring
	closest := h.ClosestNodeInFingerTable(node_id, self) //finds the closest node in the finger table that is <= key
	if closest == -1 {
		//create a NodeAddress element to send
		NodeAddress := h.Node{
			Id:      self.Id,
			Address: self.Address,
		}
		resultChan <- NodeAddress
		return
	} else {
		fmt.Printf("Node at index: %d, %s\n", closest, self.FingerTable[closest].NodeAddress)
		go func() {
			url := "http://" + self.Neighbours.Predecessor.Address + "/node-id/" + strconv.Itoa(node_id) //url of the request
			wg.Add(1)
			response, err, statusCode := h.GetRequest(url)
			if err != nil {
				h.HandleError(err, "Could not find the node id", false)
				resultChan <- h.Node{}
				return
			}
			wg.Done()
			if statusCode == http.StatusInternalServerError {
				wg.Add(1)
				go func() {
					FindPrevious(self.FingerTable[closest].NodeId, self)
					wg.Done()
				}()
				wg.Wait()
				FindNodeId(node_id, self, resultChan)
				return
			}
			wg.Wait()
			NodeAddress := h.Node{}
			json.Unmarshal([]byte(response), &NodeAddress)
			resultChan <- NodeAddress
		}()
	}
}

func RTT(Address string) int64 {

	startTime := time.Now()
	//take time here
	resp, err := http.Get(Address)
	if err != nil {
		return -1
	}

	defer resp.Body.Close()
	endTime := time.Now()
	RTT := endTime.Sub(startTime).Milliseconds()

	return RTT
}



func FindSuccessor(self *h.Node) h.Node {
	// Find the successor of a node with a specific id
	return *self
}

func NodeJoined() { //Node joined the network, called locally when THIS node is joining the network

}

func NodeJoinedGossip() { //Node joined the network, called remotely when ANOTHER node is joining the network

}

func BalanceChord() {

}

func GotData() {

}

func GotDecryptionKey() {

}

func AttatchNodeToSeed(self *h.Node, node h.RTTEntry){

    var bestRTT uint64 = math.MaxUint64
    var index int = -1 
    for i, attatchedNodes := range self.NodesToSendTo {  //iterate over and find the node that has highest RTT and replace that if lower RTT
        if attatchedNodes.Address == node.Address {
            return
        }
        if attatchedNodes.RTTTime < bestRTT {
            bestRTT = attatchedNodes.RTTTime
            index = i
        }
    }
    if len(self.NodesToSendTo) < 3 {
        self.NodesToSendTo = append(self.NodesToSendTo, node)
    } else {
        if index != -1 { //found a worse time, replace the worst node
            //send request to the node to tell it to find new 
            resp, err := http.Get("http://" + self.NodesToSendTo[index].Address + "/RemoveNodeFromSeed/" + self.Address)
            if err != nil {
                log.Fatalf("Error on get request\n Errormsg: %s", err)
            }
            defer resp.Body.Close()

            self.NodesToSendTo[index] = node
        }
    }
}

func FindClosestSeed(self *h.Node, ExcludedSeed string) {
    var bestRTT uint64 = math.MaxUint64
    var bestSeed string
    for _, seedAddress := range self.Seeds {
        if seedAddress == ExcludedSeed { //skip the seed we were just removed from
            continue
        }
        RTT := h.GetRTT(seedAddress)      
        if RTT < bestRTT {
            bestRTT = RTT
            bestSeed = seedAddress  
        }
    }
    self.SeedToReceiveFrom.Address = bestSeed 
    self.SeedToReceiveFrom.RTTTime = bestRTT
}



func DetermineIfSeed() {

}
