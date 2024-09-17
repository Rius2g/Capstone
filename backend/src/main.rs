use axum::{
    routing::{get, post},
    Router,
    response::IntoResponse,
    extract::{Json, State},
};
use rsa::{
    RsaPrivateKey, RsaPublicKey,
    pkcs8::{EncodePrivateKey, LineEnding},
};
use serde_json::Value;
use rand::rngs::OsRng;
use hyper::http::StatusCode;
use std::error::Error;
use web3::{transports::Http, Web3};
use std::sync::Arc;
use tokio::sync::Mutex; 
use std::net::SocketAddr;
use std::str::FromStr;
use std::collections::HashMap;
use serde::{Deserialize, Serialize};
use web3::contract::{Contract, Options};
use web3::types::{Address, FilterBuilder, Log, U256, Bytes};
use web3::ethabi::{Event, RawLog};
use hex::{decode, encode};
use anyhow::{Result, anyhow};
use tiny_keccak::{Keccak, Hasher};

#[derive(Debug, Serialize, Deserialize, Clone)]
struct PushEncryptedDataEvent { 
    encrypted_data: Vec<u8>,
    owner: String,
    data_name: String,
    data_hash: Vec<u8>,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
struct PushPrivateKeyEvent { 
    private_key: Vec<u8>,
    owner: String,
    data_name: String,
    data_hash: Vec<u8>, 
}

fn decrypt_data(encrypted_data: &[u8], private_key: &[u8]) -> Vec<u8> {
    encrypted_data.iter()
        .zip(private_key.iter())
        .map(|(a, b)| a ^ b)
        .collect()
}

async fn ping() -> &'static str {
    "pong"
}


#[derive(Deserialize)]
struct BroadCastData {
    data: String, 
    owner: String,
    data_name: String,
    release_time: String,
}

fn keccak256(data: &[u8]) -> [u8; 32] {
    let mut hasher = Keccak::v256();
    hasher.update(data);
    let mut output = [0u8; 32];
    hasher.finalize(&mut output);
    output
}

type SharedState = Arc<Mutex<HashMap<Vec<u8>, (PushEncryptedDataEvent, Option<PushPrivateKeyEvent>)>>>;
type ContractState = Arc<Contract<Http>>;
type Web3State = Arc<Web3<Http>>;

struct AppState {
    shared_state: SharedState,
    web3: Web3State,
    contract: ContractState,
}

async fn post_data(
    State(state): State<Arc<AppState>>,
    Json(payload): Json<BroadCastData>,
) -> impl IntoResponse {
    let data = payload.data; 
    let owner = payload.owner;
    let data_name = payload.data_name;
    let release_time = payload.release_time;
    let data_hash = keccak256(data.as_bytes());

    let (encrypted_data, private_key) = match generate_rsa_keypair_and_encrypt(data.as_bytes()) {
        Ok((encrypted_data, private_key)) => (encrypted_data, private_key),
        Err(e) => return (StatusCode::INTERNAL_SERVER_ERROR, format!("Error encrypting data: {:?}", e)).into_response(),
    };

    let encrypted_data_bytes = Bytes::from(encrypted_data);
    let private_key_bytes = Bytes::from(private_key.into_bytes());

    let release_time_u256 = match U256::from_dec_str(&release_time) {
        Ok(time) => time,
        Err(e) => return (StatusCode::BAD_REQUEST, format!("Invalid release time: {:?}", e)).into_response(),
    };

    let data_hash_bytes = Bytes::from(data_hash.to_vec());

    let accounts = match state.web3.eth().accounts().await {
        Ok(accs) => accs,
        Err(e) => return (StatusCode::INTERNAL_SERVER_ERROR, format!("Failed to get accounts: {:?}", e)).into_response(),
    };

    if accounts.is_empty() {
        return (StatusCode::INTERNAL_SERVER_ERROR, "No accounts available".to_string()).into_response();
    }

    let from_account = accounts[0];

    let params = (
        encrypted_data_bytes,
        private_key_bytes,
        owner,
        data_name,
        release_time_u256,
        data_hash_bytes
    );

    match state.contract.call("addStoredData", params, from_account, Options::default()).await {
        Ok(tx_hash) => {
            println!("Transaction sent: {:?}", tx_hash);
            (StatusCode::OK, format!("Data broadcasted successfully! Transaction hash: {:?}", tx_hash)).into_response()
        },
        Err(e) => {
            (StatusCode::INTERNAL_SERVER_ERROR, format!("Error sending transaction: {:?}", e)).into_response()
        }
    }
}

fn generate_rsa_keypair_and_encrypt(data: &[u8]) -> Result<(Vec<u8>, String), Box<dyn Error>> {
    let mut rng = OsRng;
    let bits = 2048;
    let private_key = RsaPrivateKey::new(&mut rng, bits)?;
    let public_key = RsaPublicKey::from(&private_key);

    let encrypted_data = public_key.encrypt(&mut rng, rsa::Pkcs1v15Encrypt, data)?;

    let private_key_pem = private_key.to_pkcs8_pem(LineEnding::LF)?;

    Ok((encrypted_data, private_key_pem.to_string()))
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    println!("Hello, world!");

    let shared_state: SharedState = Arc::new(Mutex::new(HashMap::new()));

    let avalanche_rpc = "https://api.avax-test.network/ext/bc/C/rpc";
    let transport = Http::new(avalanche_rpc)?;
    let web3 = Web3State::new(Web3::new(transport));

    let contract_address = Address::from_str("YOUR_CONTRACT_ADDRESS_HERE")?;
    let contract_abi = load_abi()?;
    let contract: ContractState = Arc::new(Contract::from_json(
        web3.eth(),
        contract_address,
        contract_abi.as_bytes(),
    )?);

    let app_state = Arc::new(AppState {
        shared_state: shared_state.clone(),
        web3: web3.clone(),
        contract: contract.clone(),
    });

    tokio::spawn({
        let app_state = app_state.clone();
        async move {
            if let Err(e) = run_web3_listener(app_state).await {
                eprintln!("Error in web3 listener: {:?}", e);
            }
        }
    });

    let app = Router::new()
        .route("/data/:hash", get(get_data))
        .route("/ping", get(ping))
        .route("/post_data", post(post_data))
        .with_state(app_state);

    let addr = SocketAddr::from(([127, 0, 0, 1], 8080));
    println!("Listening on http://{}", addr);

    axum::Server::bind(&addr)
        .serve(app.into_make_service())
        .await?;

    Ok(())
}

fn load_abi() -> Result<String, Box<dyn std::error::Error>> {
    let file_path = "../TwoPhaseCommit.json";
    let contract_json: Value = serde_json::from_str(&std::fs::read_to_string(file_path)?)?;

    let abi = contract_json["abi"].to_string();

    Ok(abi)
}

async fn run_web3_listener(app_state: Arc<AppState>) -> Result<()> {
    let filter = FilterBuilder::default()
        .address(vec![app_state.contract.address()])
        .from_block(web3::types::BlockNumber::Latest)
        .build();

    let push_encrypted_data_event = app_state.contract.abi().event("PushEncryptedData")?.clone();
    let push_private_key_event = app_state.contract.abi().event("PushPrivateKey")?.clone();

    println!("Web3 listener started...");

    loop {
        match app_state.web3.eth().logs(filter.clone()).await {
            Ok(logs) => {
                for log in logs {
                    if let Err(e) = process_log(log, &push_encrypted_data_event, &push_private_key_event, &app_state.shared_state).await {
                        println!("Error processing log: {:?}", e);
                    }
                }
            }
            Err(e) => println!("Error fetching logs: {:?}", e),
        }
        tokio::time::sleep(tokio::time::Duration::from_secs(5)).await;
    }
}

async fn process_log(log: Log, push_encrypted_data_event: &Event, push_private_key_event: &Event, shared_state: &SharedState) -> Result<()> {
    let raw_log = RawLog {
        topics: log.topics,
        data: log.data.0,
    };

    if let Ok(parsed_log) = push_encrypted_data_event.parse_log(raw_log.clone()) {
        let encrypted_data = parsed_log.params[0].value.clone().into_bytes().ok_or_else(|| anyhow!("Failed to parse encrypted data"))?;
        let owner = parsed_log.params[1].value.clone().into_string().ok_or_else(|| anyhow!("Failed to parse owner"))?;
        let data_name = parsed_log.params[2].value.clone().into_string().ok_or_else(|| anyhow!("Failed to parse data name"))?;
        let data_hash = parsed_log.params[3].value.clone().into_bytes().ok_or_else(|| anyhow!("Failed to parse data hash"))?;

        let encrypted_data_event = PushEncryptedDataEvent {
            encrypted_data,
            owner,
            data_name,
            data_hash: data_hash.clone(),
        };

        println!("Received encrypted data for: {}", encrypted_data_event.data_name);
        let mut state = shared_state.lock().await;
        state.entry(data_hash)
            .and_modify(|e| e.0 = encrypted_data_event.clone())
            .or_insert((encrypted_data_event, None));
    } else if let Ok(parsed_log) = push_private_key_event.parse_log(raw_log) {
        let private_key = parsed_log.params[0].value.clone().into_bytes().ok_or_else(|| anyhow!("Failed to parse private key"))?;
        let owner = parsed_log.params[1].value.clone().into_string().ok_or_else(|| anyhow!("Failed to parse owner"))?;
        let data_name = parsed_log.params[2].value.clone().into_string().ok_or_else(|| anyhow!("Failed to parse data name"))?;
        let data_hash = parsed_log.params[3].value.clone().into_bytes().ok_or_else(|| anyhow!("Failed to parse data hash"))?;

        let private_key_event = PushPrivateKeyEvent {
            private_key,
            owner,
            data_name,
            data_hash: data_hash.clone(),
        };

        println!("Received private key for: {}", private_key_event.data_name);
        let mut state = shared_state.lock().await;
        if let Some(entry) = state.get_mut(&data_hash) {
            entry.1 = Some(private_key_event.clone());
            println!("Data '{}' is ready for decryption!", private_key_event.data_name);
        } else {
            println!("Received private key for unknown data: {}", private_key_event.data_name);
        }
    }

    Ok(())
}

async fn get_data(
    axum::extract::Path(hash): axum::extract::Path<String>,
    State(state): State<Arc<AppState>>,
) -> impl IntoResponse {
    let state = state.shared_state.lock().await;
    let hash_bytes = decode(&hash).unwrap_or_default();
    
    for (stored_hash, (encrypted_data, private_key)) in state.iter() {
        if stored_hash == &hash_bytes {
            return match private_key {
                Some(key) => {
                    let decrypted_data = decrypt_data(&encrypted_data.encrypted_data, &key.private_key);
                    (StatusCode::OK, format!(
                        "Data: {}\nOwner: {}\nDecrypted: {}",
                        encrypted_data.data_name,
                        encrypted_data.owner,
                        String::from_utf8_lossy(&decrypted_data)
                    )).into_response()
                },
                None => (StatusCode::OK, format!(
                    "Data: {}\nOwner: {}\nEncrypted: {}\nKey: Not yet received",
                    encrypted_data.data_name,
                    encrypted_data.owner,
                    encode(&encrypted_data.encrypted_data)
                )).into_response(),
            };
        }
    }
    
    (StatusCode::NOT_FOUND, format!("No data found for hash {}", hash)).into_response()
}