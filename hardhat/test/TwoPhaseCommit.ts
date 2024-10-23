import { ethers } from "hardhat";
import { expect } from "chai";
import { TwoPhaseCommit } from "../typechain-types";
import { time } from "@nomicfoundation/hardhat-network-helpers";
import { encodeBytes32String, toUtf8Bytes, keccak256, hexlify } from "ethers";

describe("TwoPhaseCommit", function () {
  let twoPhaseCommit: TwoPhaseCommit;

  beforeEach(async function() {
    const TwoPhaseCommitFactory = await ethers.getContractFactory("TwoPhaseCommit");
    twoPhaseCommit = await TwoPhaseCommitFactory.deploy() as TwoPhaseCommit;
    await twoPhaseCommit.waitForDeployment();
  });

  it("Should add stored data and retrieve it successfully", async function () {
    const stringValue = "Hello, World!";
    const bytesValue = toUtf8Bytes(stringValue);
    const hash = keccak256(bytesValue);
    const releaseTime = (await time.latest()) + 86400; // 24 hours from now

    const testData = {
      encryptedData: bytesValue,
      owner: "Alice",
      dataName: "TestData",
      releaseTime: releaseTime,
      hash: hash,
    };

    await twoPhaseCommit.addStoredData(
      testData.encryptedData,
      testData.owner,
      testData.dataName,
      testData.releaseTime,
      testData.hash
    );

    const storedData = await twoPhaseCommit.returnStoredData();
    expect(storedData.length).to.equal(1);
    const firstStoredData = storedData[0];

    expect(hexlify(firstStoredData.encryptedData)).to.equal(hexlify(testData.encryptedData));
    expect(firstStoredData.owner).to.equal(testData.owner);
    expect(firstStoredData.dataName).to.equal(testData.dataName);
    expect(firstStoredData.releaseTime).to.equal(BigInt(testData.releaseTime));
    expect(firstStoredData.keyReleased).to.equal(false);
    expect(firstStoredData.hash).to.equal(testData.hash);
  });

  it("Should add several stored data entries and retrieve them successfully", async function () {
    const currentTime = await time.latest();
    const testDataArray = [
      {
        encryptedData: toUtf8Bytes("EncryptedHello"),
        owner: "Bob",
        dataName: "TestData2",
        releaseTime: currentTime + 86400,
        hash: keccak256(toUtf8Bytes("EncryptedHello")),
      },
      {
        encryptedData: toUtf8Bytes("EncryptedWorld"),
        owner: "Alice",
        dataName: "TestData3",
        releaseTime: currentTime + 172800,
        hash: keccak256(toUtf8Bytes("EncryptedWorld")),
      },
    ];

    for (const data of testDataArray) {
      await twoPhaseCommit.addStoredData(
        data.encryptedData,
        data.owner,
        data.dataName,
        data.releaseTime,
        data.hash
      );
    }

    const storedData = await twoPhaseCommit.returnStoredData();
    expect(storedData.length).to.equal(2);

    for (let i = 0; i < storedData.length; i++) {
      expect(hexlify(storedData[i].encryptedData)).to.equal(hexlify(testDataArray[i].encryptedData));
      expect(storedData[i].owner).to.equal(testDataArray[i].owner);
      expect(storedData[i].dataName).to.equal(testDataArray[i].dataName);
      expect(storedData[i].releaseTime).to.equal(BigInt(testDataArray[i].releaseTime));
      expect(storedData[i].keyReleased).to.equal(false);
      expect(storedData[i].hash).to.equal(testDataArray[i].hash);
    }
  });


  it("Should emit 'KeyReleaseRequested' event when approaching release time", async function () {
    const currentTime = await time.latest();
    const releaseTime = currentTime + 86400; // 24 hours from now

    const testData = {
      encryptedData: toUtf8Bytes("EncryptedHello"),
      owner: "Alice",
      dataName: "TestData",
      releaseTime: releaseTime,
      hash: keccak256(toUtf8Bytes("EncryptedHello")),
    };

    await twoPhaseCommit.addStoredData(
      testData.encryptedData,
      testData.owner,
      testData.dataName,
      testData.releaseTime,
      testData.hash
    );

    // Move time to 12 hours before release time (43200 seconds)
    await time.increaseTo(releaseTime - 43200);
    
    const checkData = "0x";
    const { upkeepNeeded, performData } = await twoPhaseCommit.checkUpkeep.staticCall(checkData);

    expect(upkeepNeeded).to.equal(true);

    const tx = await twoPhaseCommit.performUpkeep(performData);
    await expect(tx)
      .to.emit(twoPhaseCommit, "KeyReleaseRequested")
      .withArgs(0, testData.owner, testData.dataName);

    // Check that phase is now 1
    const storedData = await twoPhaseCommit.returnStoredData();
    expect(storedData[0].phase).to.equal(1);

    // Move time to actual release time and check second phase
    await time.increaseTo(releaseTime + 1);
    
    const { upkeepNeeded: secondUpkeepNeeded } = await twoPhaseCommit.checkUpkeep.staticCall(checkData);
    expect(secondUpkeepNeeded).to.equal(true);

    const secondTx = await twoPhaseCommit.performUpkeep(performData);
    await expect(secondTx)
      .to.emit(twoPhaseCommit, "KeyReleased")
      .withArgs(hexlify("0x"), testData.owner, testData.dataName);

    // Check that phase is now 2
    const finalStoredData = await twoPhaseCommit.returnStoredData();
    expect(finalStoredData[0].phase).to.equal(2);
  });

  
  it("Should revert when adding data with empty owner", async function () {
    const currentTime = await time.latest();
    const testData = {
      encryptedData: toUtf8Bytes("Some data"),
      owner: "",
      dataName: "TestData",
      releaseTime: currentTime + 86400,
      hash: keccak256(toUtf8Bytes("Some data")),
    };

    await expect(
      twoPhaseCommit.addStoredData(
        testData.encryptedData,
        testData.owner,
        testData.dataName,
        testData.releaseTime,
        testData.hash
      )
    ).to.be.revertedWith("Owner is required");
  });

  it("Should revert when adding data with empty dataName", async function () {
    const currentTime = await time.latest();
    const testData = {
      encryptedData: toUtf8Bytes("Some data"),
      owner: "Alice",
      dataName: "",
      releaseTime: currentTime + 86400,
      hash: keccak256(toUtf8Bytes("Some data")),
    };

    await expect(
      twoPhaseCommit.addStoredData(
        testData.encryptedData,
        testData.owner,
        testData.dataName,
        testData.releaseTime,
        testData.hash
      )
    ).to.be.revertedWith("Data name is required");
  });

  it("Should revert when adding data with past releaseTime", async function () {
    const currentTime = await time.latest();
    const testData = {
      encryptedData: toUtf8Bytes("Some data"),
      owner: "Alice",
      dataName: "TestData",
      releaseTime: currentTime - 86400,
      hash: keccak256(toUtf8Bytes("Some data")),
    };

    await expect(
      twoPhaseCommit.addStoredData(
        testData.encryptedData,
        testData.owner,
        testData.dataName,
        testData.releaseTime,
        testData.hash
      )
    ).to.be.revertedWith("Release time must be in the future");
  });

  it("Should return public data when querying by dataName and owner", async function () {
    const currentTime = await time.latest();
    const releaseTime = currentTime + 86400;

    const testData = {
      encryptedData: toUtf8Bytes("EncryptedHello"),
      owner: "Alice",
      dataName: "TestData",
      releaseTime: releaseTime,
      hash: keccak256(toUtf8Bytes("EncryptedHello")),
    };

    await twoPhaseCommit.addStoredData(
      testData.encryptedData,
      testData.owner,
      testData.dataName,
      testData.releaseTime,
      testData.hash
    );

    const publicData = await twoPhaseCommit.GetPublicData(testData.dataName, testData.owner);

    expect(hexlify(publicData[0])).to.equal(hexlify(testData.encryptedData));
    expect(publicData[1]).to.equal(testData.hash);
    expect(publicData[2]).to.equal(testData.owner);
    expect(publicData[3]).to.equal(testData.dataName);
    expect(publicData[4]).to.equal(BigInt(testData.releaseTime));
    expect(publicData[5]).to.equal(false);
  });

  afterEach(async function () {
    await twoPhaseCommit.clearStoredData();
  });
});
