import { ethers } from "hardhat";
import { expect } from "chai";
import { TwoPhaseCommit } from "../typechain-types"; // Adjust this path if needed

describe("TwoPhaseCommit", function () {
  let twoPhaseCommit: TwoPhaseCommit;

  beforeEach(async function () {
    const TwoPhaseCommit = await ethers.getContractFactory("TwoPhaseCommit");
    twoPhaseCommit = await TwoPhaseCommit.deploy();
    await twoPhaseCommit.waitForDeployment();
  });

  it("Should retrieve a recent timestamp from chainkLink", async function () {
    // Add your code here
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
      testData.releaseTime,
      "0x"
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
        testData[0].releaseTime,
        "1x"
      );

      await twoPhaseCommit.addStoredData(
        testData[1].encryptedData,
        testData[1].decryptionKey,
        testData[1].owner,
        testData[1].dataName,
        testData[1].releaseTime,
        "2x"
      );

      await twoPhaseCommit.returnStoredData(); // Remove unused variable


  })

  afterEach(async function () {
    await twoPhaseCommit.clearStoredData();
  });

  it("Should not retrieve stored data that has not been released yet", async function () {
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
      testData.releaseTime,
      "3x"
    );

    // Retrieve the stored data
    const storedData = await twoPhaseCommit.returnStoredData();
    expect(storedData.length).to.equal(1);
    const firstStoredData = storedData[0];
    expect(firstStoredData.encryptedData).to.equal(testData.encryptedData);
    expect(firstStoredData.decryptionKey).to.equal(testData.decryptionKey);
    expect(firstStoredData.owner).to.equal(testData.owner);
    expect(firstStoredData.dataName).to.deep.equal(testData.dataName);
    expect(firstStoredData.releaseTime).to.equal(testData.releaseTime);
    expect(firstStoredData.phase).to.equal(0);

    //try to retrieve the stored data before the release timestamp
  });

  })



