// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

import "@chainlink/contracts/src/v0.8/shared/interfaces/AggregatorV3Interface.sol";
import "@chainlink/contracts/src/v0.8/automation/AutomationCompatible.sol";

contract TwoPhaseCommit is AutomationCompatibleInterface {

    struct StoredData {
        string encryptedData;
        string decryptionKey; 
        string owner;
        string dataName;
        int phase; //0: data stored, 1: data is sent, 2: decryption key is sent
        int ackCount;
        uint256 releaseTime;
    }

    StoredData[] public storedData;
    AggregatorV3Interface internal priceFeed;
    uint256 public lastExecutionTime;
    uint256 public interval;

    constructor(uint256 _interval) {
         priceFeed = AggregatorV3Interface(0x5498BB86BC934c8D34FDA08E81D444153d0D06aD);
         interval = _interval;
    }

    function checkUpkeep(bytes calldata /* checkData */) external view override returns (bool upkeepNeeded, bytes memory /* performData */) {
        uint256 currentTimestamp = getLatestTimestamp();
        upkeepNeeded = (currentTimestamp - lastExecutionTime) > interval;
    }

    function performUpkeep(bytes calldata /* performData */) external override {
        uint256 currentTimestamp = getLatestTimestamp();
        if ((currentTimestamp - lastExecutionTime) > interval) {
            lastExecutionTime = currentTimestamp;
            for (uint i = 0; i < storedData.length; i++) {
                if (storedData[i].phase == 0) {
                    if (currentTimestamp - 43200 > storedData[i].releaseTime) {
                    sendEncryptedData(i); // Calling the function            
                    storedData[i].phase = 1;
                    }
                }
                else if(storedData[i].phase == 1) {
                    if (currentTimestamp >= storedData[i].releaseTime) {
                    sendDecryptionKey(i); // Calling the function
                    storedData[i].phase = 2;
                    }
                }
            }
        }
    }

    function getLatestTimestamp() public view returns (uint256) {
        (
            uint80 roundID, 
            int price,
            uint startedAt,
            uint timeStamp,
            uint80 answeredInRound
        ) = priceFeed.latestRoundData();
        return timeStamp;
    }

    function addStoredData(string memory _encryptedData, string memory _decryptionKey, string memory _owner, string memory _dataName, uint256 _releaseTime) public {
        storedData.push(StoredData(_encryptedData, _decryptionKey, _owner, _dataName, 0, 0, _releaseTime));
    }


    function returnStoredData() public view returns (StoredData[] memory){
        return storedData;
    }

    function sendEncryptedData(uint256 index) public view returns (string memory) {
        // Implementation
        //how to make the data public? How to expose the data to the public? How to hide the data from the public before this point?
        return storedData[index].encryptedData;
    }

    function sendDecryptionKey(uint256 index) public view returns (string memory) {
        // Implementation
        return storedData[index].decryptionKey;
    }
}
