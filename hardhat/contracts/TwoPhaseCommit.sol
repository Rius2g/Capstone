//contracts/TwoPhaseCommit.sol
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;
//import the chainlink client
import "@chainlink/contracts/src/v0.8/ChainlinkClient.sol";


contract TwoPhaseCommit {

    struct StoredData {
        string encryptedData;
        string decryptionKey; 
        string[] interestedParties;
        string owner;
        int phase; //0: data stored, 1: data is sent, 2: all willing have acked encrypted data 3: decryption key is sent
        int ackCount;
    }

    StoredData[] public storedData;


    constructor(){

    }

    //this contract will store the data, and the decryption key, the server will continiously check the time and see if the data should be released or not

    function addStoredData(string memory _encryptedData, string memory _decryptionKey, string memory _owner, string[] memory _interestedParties) public {
        storedData.push(StoredData(_encryptedData, _decryptionKey, _interestedParties, _owner, 0, 0));
    }


//    private function sendEncryptedData(){

  //  }

    //private function sendEncyptionKey(){

    //}





}
