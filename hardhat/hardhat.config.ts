import { HardhatUserConfig } from "hardhat/config";
import "@nomicfoundation/hardhat-toolbox";
import * as dotenv from "dotenv";
import "@nomicfoundation/hardhat-toolbox";
// or individually:
import "@nomicfoundation/hardhat-ethers";
import "@typechain/hardhat";

dotenv.config(); // Load environment variables

const config: HardhatUserConfig = {
  solidity: "0.8.19",
  networks: {
    fuji: {
      url: "https://api.avax-test.network/ext/bc/C/rpc",
      chainId: 43113,
      accounts: process.env.PRIVATE_KEY !== undefined ? [process.env.PRIVATE_KEY] : [], // Load private key from env
    },
  },
};

export default config;

