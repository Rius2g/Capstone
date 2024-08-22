// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

import "@chainlink/contracts/src/v0.8/shared/interfaces/AggregatorV3Interface.sol";
import "@chainlink/contracts/src/v0.8/automation/AutomationCompatible.sol";

contract TwoPhaseCommit is AutomationCompatibleInterface {

    struct StoredData {
        string encryptedData;
        string decryptionKey; 
        string[] interestedParties;
        string owner;
        int phase; //0: data stored, 1: data is sent, 2: all willing have acked encrypted data 3: decryption key is sent
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
            // Perform your time-based action here
            sendEncryptedData(); // Calling the function
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

    function addStoredData(string memory _encryptedData, string memory _decryptionKey, string memory _owner, string[] memory _interestedParties, uint256 _releaseTime) public {
        storedData.push(StoredData(_encryptedData, _decryptionKey, _interestedParties, _owner, 0, 0, _releaseTime));
    }

    function sendEncryptedData() public {
        // Implementation
    }
}
