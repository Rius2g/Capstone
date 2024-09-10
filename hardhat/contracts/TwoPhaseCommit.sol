// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

import "@chainlink/contracts/src/v0.8/shared/interfaces/AggregatorV3Interface.sol";

contract TwoPhaseCommit {

    struct StoredData {
        bytes encryptedData;
        bytes decryptionKey; 
        bytes hash;
        string owner;
        string dataName;
        int phase; //0: data stored, 1: data is sent, 2: decryption key is sent
        uint256 releaseTime;
    }

    struct PublicData {
        bytes data;
        bytes hash;
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

    event PushEncryptedData(bytes encryptedData, string owner, string dataName, bytes hash);
    event PushPrivateKey(bytes decryptionKey, string owner, string dataName, bytes hash);


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

    function addStoredData(bytes memory _encryptedData, bytes memory _decryptionKey, string memory _owner, string memory _dataName, uint256 _releaseTime, bytes memory _hash) public {
        require(_encryptedData.length > 0, "Encrypted data is required");
        require(_decryptionKey.length > 0, "Decryption key is required");
        require(bytes(_owner).length > 0, "Owner is required");
        require(bytes(_dataName).length > 0, "Data name is required");
        require(_releaseTime > 0, "Release time is required");
        require(_hash.length > 0, "Hash is required");
        storedData.push(StoredData(_encryptedData, _decryptionKey, _hash, _owner, _dataName, 0, _releaseTime));
    }


    function returnStoredData() public view returns (StoredData[] memory){ //testing endpoint
        return storedData;
    }

    function sendEncryptedData(uint256 index) public returns (string memory) {
        //require(getLatestTimestamp() - 43200 > storedData[index].releaseTime, "Data has not been released yet");
        require(block.timestamp > storedData[index].releaseTime - 43200, "Data has not been released yet");
        StoredData memory dataToSend = storedData[index];    
        storedData[index].phase = 1;
        emit PushEncryptedData(dataToSend.encryptedData, dataToSend.owner, dataToSend.dataName, dataToSend.hash);
    }

    function clearStoredData() public {
        delete storedData;
    }

    function sendDecryptionKey(uint256 index) public returns (string memory) {
//        require(getLatestTimestamp() - 43200 > storedData[index].releaseTime, "Decryption key has not been released yet");
        require(block.timestamp >= storedData[index].releaseTime, "Decryption key has not been released yet");

        storedData[index].phase = 2;
        StoredData memory dataToSend = storedData[index];
        emit PushPrivateKey(dataToSend.decryptionKey, dataToSend.owner, dataToSend.dataName, dataToSend.hash);
    }
}
