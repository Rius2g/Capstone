use axum::{routing::get, routing::post, extract::Json, extract::State, Router, response::IntoResponse};
use std::net::SocketAddr;
use serde::Deserialize;
use serde_json::json;
use hyper::Server; // Use hyper's Server
use web3::transports::Http;
use web3::Web3;
use std::sync::Arc; 
// use ethers::prelude::*;
// use ethers::providers::{Provider, Http};
//
struct AppState {
    web3: Web3<Http>,
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>>{  
    load_env().expect("Failed to load .env file");
    const AVALANCHE_URL: &str = "https://api.avax.network/ext/bc/C/rpc";
    let transport = Http::new(AVALANCHE_URL)?;
    let web3 = Web3::new(transport);
    let shared_state = Arc::new(AppState { web3 });
    // Connect to the network
    let app = Router::new()
        .route("/get_data", get(get_specific_from_blockchain))
        .route("/get_all_kvps", get(get_all_from_blockchain))
        .route("/post_data", post(post_to_blockchain))
        .route("/get_block_data", get(|State(state): State<Arc<AppState>>| async move {
            get_block_data(state).await
        }))
        .with_state(shared_state);
    

    let addr = SocketAddr::from(([127, 0, 0, 1], 3000));
    println!("Listening on {}", addr);
    
    // Use hyper's Server to bind and serve
    Server::bind(&addr)
        .serve(app.into_make_service())
        .await
        .unwrap();
    Ok(())
}

//endpoints we need
//1. submitting the data to be stored on the blockchain, needs data and a timestamp for when available
//2. getting the data from the blockchain when the time comes
//more endpoints will be added as needed 

#[derive(Deserialize, Debug)]
struct StoredData {
    data: String,
    timestamp: String,
}


async fn get_block_data(state: Arc<AppState>) -> impl IntoResponse {
    match fetch_block_data(state).await {
        Ok(block_data) => Json(block_data).into_response(),
        Err(e) => (
            axum::http::StatusCode::INTERNAL_SERVER_ERROR,
            format!("Error fetching block data: {}", e)
        ).into_response()
    }
}


async fn fetch_block_data(state: Arc<AppState>) -> web3::Result<serde_json::Value> {
    let block_number = state.web3.eth().block_number().await?;
    let block = state.web3.eth().block(web3::types::BlockId::Number(block_number.into())).await?;

    Ok(json!({
        "block_number": block_number,
        "block_hash": block.as_ref().and_then(|b| b.hash),
        "timestamp": block.as_ref().and_then(|b| Some(b.timestamp.as_u64())),
        "transactions_count": block.as_ref().map(|b| b.transactions.len()),
        "gas_used": block.as_ref().and_then(|b| Some(b.gas_used)),
    }))
}
async fn post_to_blockchain(Json(payload): Json<StoredData>) -> impl IntoResponse {
    println!("Received data: {:?}", payload);

    //handle the data here, we need to encrypt the data and store it on the blockchain, we also need to handle the key somehow


    // Simulate posting to blockchain and return a response
    "Data successfully posted to the blockchain"
}


#[derive(Deserialize, Debug)]
struct DataRequest {
    key: String,
}
async fn get_specific_from_blockchain() -> impl IntoResponse {
    //check if the data is decrypted, if so return it, if not return the time left to fetch
    //do some web3 stuff here?
    "Data successfully retrieved from the blockchain"
}

async fn get_all_from_blockchain() -> impl IntoResponse {
    //return all the data that we have stored on the blockchain back, with timestamps and all
    "Data successfully retrieved from the blockchain"
}


fn load_env() -> dotenv::Result<()> {
    dotenv::dotenv().ok();
    dotenv::from_filename(".env").ok();
    Ok(())
}
