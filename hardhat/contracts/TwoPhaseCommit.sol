// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;
import "@chainlink/contracts/src/v0.8/automation/interfaces/AutomationCompatibleInterface.sol";

contract TwoPhaseCommit is AutomationCompatibleInterface {
    struct StoredData {
        bytes encryptedData;
        bytes hash;
        string owner;
        string dataName;
        uint256 releaseTime;
        bool keyReleased;
        int phase;
    }
    
    StoredData[] public storedData;
    
    event ReleaseEncryptedData(
        bytes encryptedData,
        string owner,
        string dataName,
        uint256 releaseTime,
        bytes hash
    );
    event KeyReleaseRequested(uint256 index, string owner, string dataName);
    event KeyReleased(bytes privateKey, string owner, string dataName);
    
    function addStoredData(
        bytes memory _encryptedData,
        string memory _owner,
        string memory _dataName,
        uint256 _releaseTime,
        bytes memory _hash
    ) 
        public     
        ValidInput(_encryptedData, _owner, _dataName, _releaseTime, _hash) 
    {
        storedData.push(StoredData({
            encryptedData: _encryptedData,
            hash: _hash,
            owner: _owner,
            dataName: _dataName,
            releaseTime: _releaseTime,
            keyReleased: false,
            phase: 0
        }));
    }

    function returnStoredData() public view returns (StoredData[] memory) {
        return storedData;
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
                storedData[i].phase = 1;
                emit ReleaseEncryptedData(
                    storedData[i].encryptedData,
                    storedData[i].owner,
                    storedData[i].dataName,
                    storedData[i].releaseTime,
                    storedData[i].hash
                );
            } else if (storedData[i].phase == 1 && storedData[i].releaseTime <= block.timestamp) {
                storedData[i].phase = 2;
                emit KeyReleased(new bytes(0), storedData[i].owner, storedData[i].dataName);
            }
        }
    }
    
    
    function GetPublicData(string memory _dataName, string memory _owner) public view returns (
        bytes memory,
        bytes memory,
        string memory,
        string memory,
        uint256,
        bool
    ) {
        for (uint i = 0; i < storedData.length; i++) {
            if (
                keccak256(abi.encodePacked(storedData[i].dataName)) == keccak256(abi.encodePacked(_dataName)) &&
                keccak256(abi.encodePacked(storedData[i].owner)) == keccak256(abi.encodePacked(_owner))
            ) {
                return (
                    storedData[i].encryptedData,
                    storedData[i].hash,
                    storedData[i].owner,
                    storedData[i].dataName,
                    storedData[i].releaseTime,
                    storedData[i].keyReleased
                );
            }
        }
        return (new bytes(0), new bytes(0), "", "", 0, false);
    }

    function releaseKey(string memory _dataName, string memory _owner, bytes memory _privateKey) public {
        for (uint i = 0; i < storedData.length; i++) {
            if (
                keccak256(abi.encodePacked(storedData[i].dataName)) == keccak256(abi.encodePacked(_dataName)) &&
                keccak256(abi.encodePacked(storedData[i].owner)) == keccak256(abi.encodePacked(_owner))
            ) {
                require(!storedData[i].keyReleased, "Key already released");
                storedData[i].keyReleased = true;
                emit KeyReleased(_privateKey, _owner, _dataName);
                break;
            }
        }
    }
    
    modifier ValidInput(
        bytes memory _encryptedData,
        string memory _owner,
        string memory _dataName,
        uint256 _releaseTime,
        bytes memory _hash
    ) {
        require(_encryptedData.length > 0, "Encrypted data is required");
        require(bytes(_owner).length > 0, "Owner is required");
        require(bytes(_dataName).length > 0, "Data name is required");
        require(_releaseTime > block.timestamp, "Release time must be in the future");
        require(_hash.length > 0, "Hash is required");
        (,,,, uint256 releaseTime,) = GetPublicData(_dataName, _owner);
        require(releaseTime == 0, "Data name and owner combination already exists");
        _;
    }
    
    function clearStoredData() public {
        delete storedData;
    }
}
