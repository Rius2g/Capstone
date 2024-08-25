import { ethers } from "hardhat";
import { expect } from "chai";
import { TwoPhaseCommit } from "../typechain-types"; // Adjust this path if needed
import { experimentalAddHardhatNetworkMessageTraceHook } from "hardhat/config";

describe("TwoPhaseCommit", function () {
  let twoPhaseCommit: TwoPhaseCommit;
  const interval = 3600; // 1 hour in seconds

  beforeEach(async function () {
    const TwoPhaseCommit = await ethers.getContractFactory("TwoPhaseCommit");
    twoPhaseCommit = await TwoPhaseCommit.deploy(interval);
    await twoPhaseCommit.waitForDeployment();
  it("Should retrieve a recent timestamp from chainkLink", async function () {
    // Add your code here
  });
});

  it("Should add stored data and retrieve it successfully", async function () {
    const testData = {
      encryptedData: "EncryptedHello",
      decryptionKey: "SecretKey123",
      owner: "Alice",
      dataName: "TestData",
      releaseTime: Math.floor(Date.now() / 1000) + 86400 // 24 hours from now
    };

    // Add the stored data
    await twoPhaseCommit.addStoredData(
      testData.encryptedData,
      testData.decryptionKey,
      testData.owner,
      testData.dataName,
      testData.releaseTime
    );

    // Retrieve the stored data
    const storedData = await twoPhaseCommit.returnStoredData();

    // Check if the data was stored correctly
    expect(storedData.length).to.equal(1);
    const firstStoredData = storedData[0];

    expect(firstStoredData.encryptedData).to.equal(testData.encryptedData);
    expect(firstStoredData.decryptionKey).to.equal(testData.decryptionKey);
    expect(firstStoredData.owner).to.equal(testData.owner);
    expect(firstStoredData.dataName).to.deep.equal(testData.dataName);
    expect(firstStoredData.releaseTime).to.equal(testData.releaseTime);
    expect(firstStoredData.phase).to.equal(0);
  });


  it("Should add several stored data and retrieve them successfully", async function () {
    const testData = [
      {
        encryptedData: "EncryptedHello",
        decryptionKey: "SecretKey12",
        owner: "Bob",
        dataName: "TestData2",
        releaseTime: Math.floor(Date.now() / 1000) + 86400 // 24 hours from now
      },

      {
        encryptedData: "EncryptedWorld",
        decryptionKey: "SecretKey123",
        owner: "Alice",
        dataName: "TestData3",
        releaseTime: Math.floor(Date.now() / 1000) + 86400 // 24 hours from now
      }
    ];


      await twoPhaseCommit.addStoredData(
        testData[0].encryptedData,
        testData[0].decryptionKey,
        testData[0].owner,
        testData[0].dataName,
        testData[0].releaseTime
      );

      await twoPhaseCommit.addStoredData(
        testData[1].encryptedData,
        testData[1].decryptionKey,
        testData[1].owner,
        testData[1].dataName,
        testData[1].releaseTime
      );

      const allStoredData = await twoPhaseCommit.returnStoredData();

    expect(allStoredData.length).to.equal(2);  

    expect(allStoredData[0].encryptedData).to.equal(testData[0].encryptedData);
    expect(allStoredData[0].decryptionKey).to.equal(testData[0].decryptionKey);
    expect(allStoredData[0].owner).to.equal(testData[0].owner);
    expect(allStoredData[0].dataName).to.deep.equal(testData[0].dataName);
    expect(allStoredData[0].releaseTime).to.equal(testData[0].releaseTime);
    expect(allStoredData[0].phase).to.equal(0);

    expect(allStoredData[1].encryptedData).to.equal(testData[1].encryptedData);
    expect(allStoredData[1].decryptionKey).to.equal(testData[1].decryptionKey);
    expect(allStoredData[1].owner).to.equal(testData[1].owner);
    expect(allStoredData[1].dataName).to.deep.equal(testData[1].dataName);
    expect(allStoredData[1].releaseTime).to.equal(testData[1].releaseTime);
    expect(allStoredData[1].phase).to.equal(0);
  
  });
});
