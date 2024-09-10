use axum::{
    routing::get,
    Router,
};
use hyper::{Body, Client, Request, Uri};
use std::net::IpAddr;
use web3::{transports::Http, Web3};
use std::sync::Arc;
use tokio::sync::Mutex; 
use std::net::SocketAddr;
use std::str::FromStr;
use std::collections::HashMap;
use serde::{Deserialize, Serialize};
use web3::contract::{Contract, Options};
use web3::types::{Address, H256, FilterBuilder, Log};
use web3::ethabi::{Event, RawLog};
use hex::{decode, encode};
use anyhow::{Result, anyhow};

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

fn decrypt_data(encrypted_data: String, private_key: String) -> String {
    let encrypted_data_bytes = hex::decode(encrypted_data).unwrap();
    let private_key_bytes = hex::decode(private_key).unwrap();

    let decrypted_data = encrypted_data_bytes.iter().zip(private_key_bytes.iter()).map(|(a, b)| a ^ b).collect::<Vec<_>>();

    hex::encode(decrypted_data)
}

async fn ping() -> &'static str {
    "pong"
}

async fn send_put_request(
    url: String,
    data: String,
) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
    let client = Client::new();
    let uri = url.parse::<Uri>()?;
    let req = Request::builder()
        .method("PUT")
        .uri(uri)
        .header("Content-Type", "text/plain")
        .body(Body::from(data))?;
    let _resp = client.request(req).await?;
    Ok(()) 
}

async fn send_get_request(
    url: String) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
    let client = Client::new();
    let uri = url.parse::<Uri>()?; 
    let req = Request::builder()
        .method("GET")
        .uri(uri)
        .body(Body::empty())?;
    let _resp = client.request(req).await?;
    Ok(())
}

fn get_local_ip() -> Option<IpAddr> {
    let socket = std::net::UdpSocket::bind("0.0.0.0:0").ok()?;
    socket.connect("8.8.8.8:80").ok()?;
    Some(socket.local_addr().ok()?.ip())
}

type SharedState = Arc<Mutex<HashMap<Vec<u8>, (PushEncryptedDataEvent, Option<PushPrivateKeyEvent>)>>>;

#[tokio::main]
async fn main() -> Result<()> {
    println!("Hello, world!");

    let shared_state: SharedState = Arc::new(Mutex::new(HashMap::new()));
    let web3_state = shared_state.clone();

    tokio::spawn(async move {
        if let Err(e) = run_web3_listener(web3_state).await {
            eprintln!("Error in web3 listener: {:?}", e);
        }
    });

    let app_state = shared_state.clone();

    let app = Router::new()
        .route("/ping", get(ping))
        .route("/data/:hash", get(get_data))
        .with_state(app_state);

   let addr = SocketAddr::from(([127, 0, 0, 1], 8080));
   println!("Listening on http://{}", addr);

   axum::Server::bind(&addr)
       .serve(app.into_make_service())
       .await?;

    Ok(())
}

async fn run_web3_listener(shared_state: SharedState) -> Result<()> {
    let avalanche_rpc = "https://api.avax-test.network/ext/bc/C/rpc"; 
    let transport = Http::new(avalanche_rpc)?;
    let web3 = Web3::new(transport);

    // Replace with your actual contract address
    let contract_address = Address::from_str("0x123456789...")?;

    // ABI for both events (adjust according to your actual contract ABI)
    let contract_abi: &[u8] = br#"[
        {"anonymous":false,"inputs":[{"indexed":false,"name":"encryptedData","type":"bytes"},{"indexed":false,"name":"owner","type":"string"},{"indexed":false,"name":"dataName","type":"string"},{"indexed":false,"name":"hash","type":"bytes"}],"name":"PushEncryptedData","type":"event"},
        {"anonymous":false,"inputs":[{"indexed":false,"name":"decryptionKey","type":"bytes"},{"indexed":false,"name":"owner","type":"string"},{"indexed":false,"name":"dataName","type":"string"},{"indexed":false,"name":"hash","type":"bytes"}],"name":"PushPrivateKey","type":"event"}
    ]"#;

    let contract = Contract::from_json(web3.eth(), contract_address, contract_abi)?;

    let filter = FilterBuilder::default()
        .address(vec![contract_address])
        .from_block(web3::types::BlockNumber::Latest)
        .build();

    let push_encrypted_data_event = contract.abi().event("PushEncryptedData")?.clone();
    let push_private_key_event = contract.abi().event("PushPrivateKey")?.clone();

    println!("Web3 listener started...");

    tokio::spawn(async move {
        loop {
            match web3.eth().logs(filter.clone()).await {
                Ok(logs) => {
                    for log in logs {
                        if let Err(e) = process_log(log, &push_encrypted_data_event, &push_private_key_event, &shared_state).await {
                            println!("Error processing log: {:?}", e);
                        }
                    }
                }
                Err(e) => println!("Error fetching logs: {:?}", e),
            }
            tokio::time::sleep(tokio::time::Duration::from_secs(5)).await;
        }
    });

    Ok(())

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
    axum::extract::State(state): axum::extract::State<SharedState>,
) -> String {
    let state = state.lock().await;
    let hash_bytes = hex::decode(&hash).unwrap_or_default();
    
    for (stored_hash, (encrypted_data, private_key)) in state.iter() {
        if stored_hash == &hash_bytes {
            return match private_key {
                Some(key) => {
                    let decrypted_data = decrypt_data(
                        hex::encode(&encrypted_data.encrypted_data),
                        hex::encode(&key.private_key)
                    );
                    format!(
                        "Data: {}\nOwner: {}\nDecrypted: {}",
                        encrypted_data.data_name,
                        encrypted_data.owner,
                        decrypted_data
                    )
                },
                None => format!(
                    "Data: {}\nOwner: {}\nEncrypted: {}\nKey: Not yet received",
                    encrypted_data.data_name,
                    encrypted_data.owner,
                    hex::encode(&encrypted_data.encrypted_data)
                ),
            };
        }
    }
    
    format!("No data found for hash {}", hash)
}
