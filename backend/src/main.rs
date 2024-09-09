use axum::{
    routing::get,
    Router,
};
use hyper::{Body, Client, Request, Uri};
use std::net::IpAddr;



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
    let socket = std::net::UdpSocket::bind("0.0.0.0").unwrap();

    socket.connect("0.0.0.0:80").unwrap();

    socket.local_addr().ok()?.ip().into()
}



#[tokio::main]
async fn main() {
    println!("Hello, world!");

    let app = Router::new().route("/ping", get(ping));

    axum::Server::bind(&"0.0.0.0:8080".parse().unwrap())
        .serve(app.into_make_service())
        .await
        .unwrap();
}
