package helper 

type Node struct {
    Id int
    ChordSize int
    AmountOfNodes int
    Address string
    FingerTable []FingerTableEntry //need this for effective joining of new nodes 
    Neighbours Neighbours
    RTTNeighbours RTTNeighbours
    Seed bool
}

type RTTNeighbours struct { //we can do this, since the RTT sending does not have to be a loop like the chord ring
    ClosestRTTSuccsessor RTTEntry
    RTTPredecessor RTTEntry
}

type RTTEntry struct {
    RTTNode *Node
    RTTTime int
}

type FingerTableEntry struct {
    Key int
    NodeId int
    NodeAddress string
}


type Neighbours struct {
    Predecessor *Node
    Successor *Node
}
