// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

import "@chainlink/contracts/src/v0.8/shared/interfaces/AggregatorV3Interface.sol";

contract TwoPhaseCommit {

    struct StoredData {
        string encryptedData;
        string decryptionKey; 
        string owner;
        string dataName;
        string hash;
        int phase; //0: data stored, 1: data is sent, 2: decryption key is sent
        uint256 releaseTime;
    }

    struct PublicData {
        string data;
        string owner;
        string dataName;
        uint256 id;
        uint256 releaseTime;
    }

    StoredData[] public storedData;
    AggregatorV3Interface internal priceFeed;

    constructor() {
         priceFeed = AggregatorV3Interface(0x5498BB86BC934c8D34FDA08E81D444153d0D06aD);
    }

    event PushEncryptedData(string encryptedData, string owner, string dataName);
    event PushPrivateKey(string decryptionKey, string owner, string dataName);


    function getLatestTimestamp() internal view returns (uint256) {
        (
            uint80 roundID, 
            int price,
            uint startedAt,
            uint timeStamp,
            uint80 answeredInRound
        ) = priceFeed.latestRoundData();
        return timeStamp;
    }

    function addStoredData(string memory _encryptedData, string memory _decryptionKey, string memory _owner, string memory _dataName, uint256 _releaseTime, string memory _hash) public {
        storedData.push(StoredData(_encryptedData, _decryptionKey, _owner, _dataName, _hash, 0, _releaseTime));
        index++;
    }


    function returnStoredData() public view returns (StoredData[] memory){ //testing endpoint
        return storedData;
    }

    function sendEncryptedData(uint256 index) public returns (string memory) {
        //require(getLatestTimestamp() - 43200 > storedData[index].releaseTime, "Data has not been released yet");
        require(block.timestamp - 43200 > storedData[index].releaseTime, "Data has not been released yet");
        StoredData memory dataToSend = storedData[index];    
        storedData[index].phase = 1;
        emit PushEncryptedData(dataToSend.encryptedData, dataToSend.owner, dataToSend.dataName);
    }

    function clearStoredData() public {
        delete storedData;
    }

    function sendDecryptionKey(uint256 index) public returns (string memory) {
//        require(getLatestTimestamp() - 43200 > storedData[index].releaseTime, "Decryption key has not been released yet");
        require(block.timestamp >= storedData[index].releaseTime, "Decryption key has not been released yet");

        storedData[index].phase = 2;
        StoredData memory dataToSend = storedData[index];
        emit PushPrivateKey(dataToSend.decryptionKey, dataToSend.owner, dataToSend.dataName);
    }
}
