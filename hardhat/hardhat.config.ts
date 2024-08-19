import { HardhatUserConfig } from "hardhat/config";
import "@nomicfoundation/hardhat-toolbox";

const config: HardhatUserConfig = {
  solidity: "0.8.24",
};
require('@nomiclabs/hardhat-waffle');
require('dotenv').config();

module.exports = {
  solidity: "0.8.18",
  networks: {
    fuji: {
      url: 'https://api.avax-test.network/ext/bc/C/rpc',
      accounts: [process.env.PRIVATE_KEY] // Load private key from .env file
    }
  }
};


export default config;
