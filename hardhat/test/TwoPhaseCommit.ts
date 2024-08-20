import { ethers } from "hardhat";
import { expect } from "chai";
import { TwoPhaseCommit } from "../typechain-types"; // This path might be different based on your setup

describe("TwoPhaseCommit", function () {
  let twoPhaseCommit: TwoPhaseCommit;
  const initialGreeting = "Hello, Two-Phase Commit!";
  const newGreeting = "New greeting!";

  beforeEach(async function () {
    const TwoPhaseCommit = await ethers.getContractFactory("TwoPhaseCommit");
    twoPhaseCommit = await TwoPhaseCommit.deploy(initialGreeting);
    await twoPhaseCommit.waitForDeployment();
  });

  it("Should return the initial greeting", async function () {
    expect(await twoPhaseCommit.greet()).to.equal(initialGreeting);
  });

  it("Should set a new greeting", async function () {
      await twoPhaseCommit.setGreeting(newGreeting);
      expect(await twoPhaseCommit.greet()).to.equal(newGreeting);  
  });


});
