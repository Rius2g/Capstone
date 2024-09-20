import { ethers } from "hardhat";
import { expect } from "chai";
import { TwoPhaseCommit } from "../typechain-types"; // Adjust this path if needed
import { time } from "@nomicfoundation/hardhat-network-helpers";

describe("TwoPhaseCommit", function () {
  let twoPhaseCommit: TwoPhaseCommit;

  beforeEach(async function () {
    const TwoPhaseCommit = await ethers.getContractFactory("TwoPhaseCommit");
    twoPhaseCommit = await TwoPhaseCommit.deploy();
    await twoPhaseCommit.waitForDeployment();
  });

  it("Should retrieve a recent timestamp from chainLink", async function () {
    // This test is left as a placeholder for now
    // You may want to implement it based on your contract's getLatestTimestamp function
  });

  it("Should add stored data and retrieve it successfully", async function () {
    const stringValue = "Hello, World!";
    const bytesValue = ethers.toUtf8Bytes(stringValue);
    const decryptionKey = "SecretKey123";
    const bytesKey = ethers.toUtf8Bytes(decryptionKey);
    const hash = ethers.keccak256(bytesValue);
    const testData = {
      encryptedData: bytesValue,
      decryptionKey: bytesKey, 
      owner: "Alice",
      dataName: "TestData",
      releaseTime: Date.now()+ 86400, // 24 hours from now
      hash: hash
    };

    // Add the stored data
    await twoPhaseCommit.addStoredData(
      testData.encryptedData,
      testData.decryptionKey,
      testData.owner,
      testData.dataName,
      testData.releaseTime,
      testData.hash
    );

    // Retrieve the stored data
    const storedData = await twoPhaseCommit.returnStoredData();

    // Check if the data was stored correctly
    expect(storedData.length).to.equal(1);
    const firstStoredData = storedData[0];

    expect(firstStoredData.encryptedData).to.equal(ethers.hexlify(testData.encryptedData));
    expect(firstStoredData.decryptionKey).to.equal(ethers.hexlify(testData.decryptionKey));
    expect(firstStoredData.owner).to.equal(testData.owner);
    expect(firstStoredData.dataName).to.equal(testData.dataName);
    expect(firstStoredData.releaseTime).to.equal(testData.releaseTime);
    expect(firstStoredData.phase).to.equal(0);
    expect(firstStoredData.hash).to.equal(testData.hash);
  });

  it("Should add several stored data and retrieve them successfully", async function () {
    const testData = [
      {
        encryptedData: ethers.toUtf8Bytes("EncryptedHello"),
        decryptionKey: ethers.toUtf8Bytes("SecretKey12"),
        owner: "Bob",
        dataName: "TestData2",
        releaseTime: Date.now() + 86400, // 24 hours from now
        hash: ethers.keccak256(ethers.toUtf8Bytes("EncryptedHello"))
      },
      {
        encryptedData: ethers.toUtf8Bytes("EncryptedWorld"),
        decryptionKey: ethers.toUtf8Bytes("SecretKey123"),
        owner: "Alice",
        dataName: "TestData3",
        releaseTime: Date.now() + 86400, // 24 hours from now
        hash: ethers.keccak256(ethers.toUtf8Bytes("EncryptedWorld"))
      }
    ];

    for (const data of testData) {
      await twoPhaseCommit.addStoredData(
        data.encryptedData,
        data.decryptionKey,
        data.owner,
        data.dataName,
        data.releaseTime,
        data.hash
      );
    }

    const storedData = await twoPhaseCommit.returnStoredData();
    expect(storedData.length).to.equal(2);

    for (let i = 0; i < storedData.length; i++) {
      expect(storedData[i].encryptedData).to.equal(ethers.hexlify(testData[i].encryptedData));
      expect(storedData[i].decryptionKey).to.equal(ethers.hexlify(testData[i].decryptionKey));
      expect(storedData[i].owner).to.equal(testData[i].owner);
      expect(storedData[i].dataName).to.equal(testData[i].dataName);
      expect(storedData[i].releaseTime).to.equal(testData[i].releaseTime);
      expect(storedData[i].phase).to.equal(0);
      expect(storedData[i].hash).to.equal(testData[i].hash);
    }
  });

  it("Should not retrieve stored data that has not been released yet", async function () {
    const currentTime = await time.latest();
    const testData = {
      encryptedData: ethers.toUtf8Bytes("EncryptedHello"),
      decryptionKey: ethers.toUtf8Bytes("SecretKey123"),
      owner: "Alice",
      dataName: "TestData",
      releaseTime: currentTime + 86400, // 24 hours from now
      hash: ethers.keccak256(ethers.toUtf8Bytes("EncryptedHello"))
    };

    // Add the stored data
    await twoPhaseCommit.addStoredData(
      testData.encryptedData,
      testData.decryptionKey,
      testData.owner,
      testData.dataName,
      testData.releaseTime,
      testData.hash
    );

    // Try to retrieve the encrypted data before the release time
    await expect(twoPhaseCommit.sendEncryptedData(0))
      .to.be.revertedWith("Data has not been released yet");

    // Try to retrieve the decryption key before the release time
    await expect(twoPhaseCommit.sendDecryptionKey(0))
      .to.be.revertedWith("Decryption key has not been released yet");

    // Move time forward past the release time
    await time.increaseTo(testData.releaseTime + 1);

    // Now the encrypted data should be retrievable
    await expect(twoPhaseCommit.sendEncryptedData(0))
      .to.emit(twoPhaseCommit, "PushEncryptedData")
      .withArgs(ethers.hexlify(testData.encryptedData), testData.owner, testData.dataName, testData.hash);

    // The decryption key should also be retrievable
    await expect(twoPhaseCommit.sendDecryptionKey(0))
      .to.emit(twoPhaseCommit, "PushPrivateKey")
      .withArgs(ethers.hexlify(testData.decryptionKey), testData.owner, testData.dataName, testData.hash);
  });

  it("Should emit events when sending encrypted data and decryption key", async function () {
    const currentTime = await time.latest();
    const testData = {
      encryptedData: ethers.toUtf8Bytes("EncryptedHello"),
      decryptionKey: ethers.toUtf8Bytes("SecretKey123"),
      owner: "Alice",
      dataName: "TestData",
      releaseTime: currentTime + 60, // 24
      hash: ethers.keccak256(ethers.toUtf8Bytes("EncryptedHello"))
    };

    await twoPhaseCommit.addStoredData(
      testData.encryptedData,
      testData.decryptionKey,
      testData.owner,
      testData.dataName,
      testData.releaseTime,
      testData.hash
    );

    // Send encrypted data
    await expect(twoPhaseCommit.sendEncryptedData(0))
      .to.emit(twoPhaseCommit, "PushEncryptedData")
      .withArgs(ethers.hexlify(testData.encryptedData), testData.owner, testData.dataName, testData.hash);

      //sleep 1 minute to release decryption key
      //
      await time.increaseTo(testData.releaseTime + 60);


    // Send decryption key
    await expect(twoPhaseCommit.sendDecryptionKey(0))
      .to.emit(twoPhaseCommit, "PushPrivateKey")
      .withArgs(ethers.hexlify(testData.decryptionKey), testData.owner, testData.dataName, testData.hash);
  });

  it("Should revert if encrypteddata is empty", async function () {
      const currentTime = await time.latest();
    const testData = {
      encryptedData: ethers.toUtf8Bytes(""),
      decryptionKey: ethers.toUtf8Bytes("SecretKey123"),
      owner: "Alice",
      dataName: "TestData",
      releaseTime: currentTime - 86400, // 24 hours ago
      hash: ethers.keccak256(ethers.toUtf8Bytes("Encrypted hello"))
    };

    await expect(twoPhaseCommit.addStoredData(
      testData.encryptedData,
      testData.decryptionKey,
      testData.owner,
      testData.dataName,
      testData.releaseTime,
      testData.hash
    )).to.be.revertedWith("Encrypted data is required"); 
  });


it("Should revert if Decryption key or hash is empty", async function () {
      const currentTime = await time.latest();
    const testData = {
      encryptedData: ethers.toUtf8Bytes("Testing testing"),
      decryptionKey: ethers.toUtf8Bytes(""),
      owner: "Alice",
      dataName: "TestData",
      releaseTime: currentTime - 86400, // 24 hours ago
      hash: ethers.keccak256(ethers.toUtf8Bytes("Encrypted hello"))
    };

    await expect(twoPhaseCommit.addStoredData(
      testData.encryptedData,
      testData.decryptionKey,
      testData.owner,
      testData.dataName,
      testData.releaseTime,
      testData.hash
    )).to.be.revertedWith("Decryption key is required"); 
  });




it("Should revert if owner is empty", async function () {
      const currentTime = await time.latest();
    const testData = {
      encryptedData: ethers.toUtf8Bytes("Heia lyn 123"),
      decryptionKey: ethers.toUtf8Bytes("SecretKey123"),
      owner: "",
      dataName: "TestData",
      releaseTime: currentTime - 86400, // 24 hours ago
      hash: ethers.keccak256(ethers.toUtf8Bytes("Encrypted hello"))
    };

    await expect(twoPhaseCommit.addStoredData(
      testData.encryptedData,
      testData.decryptionKey,
      testData.owner,
      testData.dataName,
      testData.releaseTime,
      testData.hash
    )).to.be.revertedWith("Owner is required"); 
  });


it("Should revert if dataname is empty", async function () {
      const currentTime = await time.latest();
    const testData = {
      encryptedData: ethers.toUtf8Bytes("Heia lyn123"),
      decryptionKey: ethers.toUtf8Bytes("SecretKey123"),
      owner: "Alice",
      dataName: "",
      releaseTime: currentTime - 86400, // 24 hours ago
      hash: ethers.keccak256(ethers.toUtf8Bytes("Encrypted hello"))
    };

    await expect(twoPhaseCommit.addStoredData(
      testData.encryptedData,
      testData.decryptionKey,
      testData.owner,
      testData.dataName,
      testData.releaseTime,
      testData.hash
    )).to.be.revertedWith("Data name is required"); 
  });


 

  it("Should return the 'public' data from the blockchain when querying for dataname + owner", async function () {
  const testData = {
    encryptedData: ethers.toUtf8Bytes("EncryptedHello"),
    decryptionKey: ethers.toUtf8Bytes("SecretKey123"),
    owner: "Alice",
    dataName: "TestData",
    releaseTime: Date.now() + 86400, // 24 hours from now, in seconds
    hash: ethers.keccak256(ethers.toUtf8Bytes("EncryptedHello"))
  };

  // Add the stored data 
  await twoPhaseCommit.addStoredData(
    testData.encryptedData,
    testData.decryptionKey,
    testData.owner,
    testData.dataName,
    testData.releaseTime,
    testData.hash
  );

  // Retrieve the stored data from the "get public data" function 
  const storedData = await twoPhaseCommit.GetPublicData(testData.dataName, testData.owner); 

  // Check the returned data
  expect(ethers.hexlify(storedData[0])).to.equal(ethers.hexlify(testData.encryptedData));
  expect(ethers.hexlify(storedData[1])).to.equal(ethers.hexlify(testData.hash));
  expect(storedData[2]).to.equal(testData.owner);
  expect(storedData[3]).to.equal(testData.dataName);
  expect(storedData[4]).to.equal(BigInt(testData.releaseTime));
});

  afterEach(async function () {
    await twoPhaseCommit.clearStoredData();
  });
});
