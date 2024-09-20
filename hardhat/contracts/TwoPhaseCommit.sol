// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

import "@chainlink/contracts/src/v0.8/shared/interfaces/AggregatorV3Interface.sol";
import "@chainlink/contracts/src/v0.8/automation/interfaces/AutomationCompatibleInterface.sol";

contract TwoPhaseCommit is AutomationCompatibleInterface {

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
        bytes encryptedData;
        bytes hash;
        string owner;
        string dataName;
        uint256 releaseTime;
    }

    StoredData[] public storedData;
    AggregatorV3Interface internal priceFeed;

    constructor() {
        priceFeed = AggregatorV3Interface(0x5498BB86BC934c8D34FDA08E81D444153d0D06aD);
    }

    function checkUpkeep(bytes calldata /* checkData */)
        external
        view
        override
        returns (bool upkeepNeeded, bytes memory performData)
    {
        upkeepNeeded = false;
        for (uint i = 0; i < storedData.length; i++) {
            if (storedData[i].phase == 0 && storedData[i].releaseTime - 43200 <= block.timestamp) {
                upkeepNeeded = true;
                break;
            } else if (storedData[i].phase == 1 && storedData[i].releaseTime <= block.timestamp) {
                upkeepNeeded = true;
                break;
            }
        }
        return (upkeepNeeded, "");
    }

    function performUpkeep(bytes calldata /* performData */) external override {
        for (uint i = 0; i < storedData.length; i++) {
            if (storedData[i].phase == 0 && storedData[i].releaseTime - 43200 <= block.timestamp) {
                sendEncryptedData(i);
            } else if (storedData[i].phase == 1 && storedData[i].releaseTime <= block.timestamp) {
                sendDecryptionKey(i);
            }
        }
    }

    event PushEncryptedData(bytes encryptedData, string owner, string dataName, bytes hash);
    event PushPrivateKey(bytes decryptionKey, string owner, string dataName, bytes hash);

    function GetPublicData(string memory _dataName, string memory _owner) public view returns (
        bytes memory,
        bytes memory,
        string memory,
        string memory,
        uint256 
    ) {
        for (uint i = 0; i < storedData.length; i++) {
            if (keccak256(abi.encodePacked(storedData[i].dataName)) == keccak256(abi.encodePacked(_dataName)) && keccak256(abi.encodePacked(storedData[i].owner)) == keccak256(abi.encodePacked(_owner))) {
                return (storedData[i].encryptedData, storedData[i].hash, storedData[i].owner, storedData[i].dataName, storedData[i].releaseTime);
            }
        }
        return (new bytes(0), new bytes(0), "", "", 0);
    }

    modifier ValidInput(bytes memory _encryptedData, bytes memory _decryptionKey, string memory _owner, string memory _dataName, uint256 _releaseTime, bytes memory _hash) {
        require(_encryptedData.length > 0, "Encrypted data is required");
        require(_decryptionKey.length > 0, "Decryption key is required");
        require(bytes(_owner).length > 0, "Owner is required");
        require(bytes(_dataName).length > 0, "Data name is required");
        require(_releaseTime > 0, "Release time is required");
        require(_hash.length > 0, "Hash is required");
        require(_releaseTime > block.timestamp, "Release time must be in the future");
        (,,,, uint256 releaseTime) = GetPublicData(_dataName, _owner);
        require(releaseTime == 0, "Data name and owner combination already exists");
        _;
    }

    function addStoredData(bytes memory _encryptedData, bytes memory _decryptionKey, string memory _owner, string memory _dataName, uint256 _releaseTime, bytes memory _hash) 
    public     
    ValidInput(_encryptedData, _decryptionKey, _owner, _dataName, _releaseTime, _hash) 
    {
        storedData.push(StoredData(_encryptedData, _decryptionKey, _hash, _owner, _dataName, 0, _releaseTime));
    }

    function returnStoredData() public view returns (StoredData[] memory){ //testing endpoint
        return storedData;
    }

    function sendEncryptedData(uint256 index) internal {
        StoredData memory dataToSend = storedData[index];    
        storedData[index].phase = 1;
        emit PushEncryptedData(dataToSend.encryptedData, dataToSend.owner, dataToSend.dataName, dataToSend.hash);
    }

    function clearStoredData() public {
        delete storedData;
    }

    function sendDecryptionKey(uint256 index) internal {
        storedData[index].phase = 2;
        StoredData memory dataToSend = storedData[index];
        emit PushPrivateKey(dataToSend.decryptionKey, dataToSend.owner, dataToSend.dataName, dataToSend.hash);
    }
}
