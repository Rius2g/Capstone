#!/bin/bash

# Configuration
API_ENDPOINT="http://localhost:8080"
OUTPUT_DIR="test_results_$(date +%Y%m%d_%H%M%S)"
ITERATIONS=5

# Create output directory
mkdir -p "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR/gas"
mkdir -p "$OUTPUT_DIR/network"

# Initialize log files
echo "Message Size (bytes),Gas Used,Transaction Hash,Block Number,Time" > "$OUTPUT_DIR/gas/gas_analysis.csv"
echo "Transaction Hash,First Node,Last Node,Time Difference (ms),Network Condition" > "$OUTPUT_DIR/network/network_analysis.csv"

# Function to generate random strings of specified size
generate_random_string() {
    local size=$1
    cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w $size | head -n 1
}

# Function to test gas usage with different message sizes
test_gas_usage() {
    local sizes=(100 1000 5000 10000 50000 100000)
    
    for size in "${sizes[@]}"; do
        for i in $(seq 1 $ITERATIONS); do
            echo "Testing message size: $size bytes (iteration $i)"
            
            # Generate random data of specified size
            data=$(generate_random_string $size)
            
            # Current timestamp as release time (24 hours from now)
            release_time=$(($(date +%s) + 86400))
            
            # Make the upload request
            response=$(curl -s -X POST "$API_ENDPOINT/upload" \
                -F "data=$data" \
                -F "owner=0xTestOwner" \
                -F "dataname=test_${size}_${i}" \
                -F "releaseTime=$release_time")
            
            # Extract relevant information
            tx_hash=$(echo $response | jq -r '.transactionHash')
            block_number=$(echo $response | jq -r '.blockNumber')
            
            # Get transaction details from blockchain
            gas_used=$(cast tx $tx_hash | grep "gas used" | awk '{print $3}')
            
            # Record results
            echo "$size,$gas_used,$tx_hash,$block_number,$(date +%s)" >> "$OUTPUT_DIR/gas/gas_analysis.csv"
            
            # Wait a bit between requests
            sleep 5
        done
    done
}

# Function to analyze network propagation
analyze_network() {
    local duration=300  # 5 minutes of monitoring
    local interval=10   # Check every 10 seconds
    
    echo "Starting network analysis for $duration seconds..."
    
    end_time=$(($(date +%s) + duration))
    
    while [ $(date +%s) -lt $end_time ]; do
        # Get network stats
        stats=$(curl -s "$API_ENDPOINT/stats")
        
        # Process each transaction's network propagation data
        echo $stats | jq -r 'to_entries[] | [.key, .value.first_node, .value.last_node, .value.time_difference_ms] | @csv' \
            >> "$OUTPUT_DIR/network/network_analysis.csv"
        
        sleep $interval
    done
}

# Function to generate network visualization
generate_network_report() {
    python3 - <<EOF
import pandas as pd
import matplotlib.pyplot as plt
import seaborn as sns

# Read the gas analysis data
gas_df = pd.read_csv('$OUTPUT_DIR/gas/gas_analysis.csv')

# Create gas usage visualization
plt.figure(figsize=(12, 6))
sns.scatterplot(data=gas_df, x='Message Size (bytes)', y='Gas Used')
plt.title('Gas Usage vs Message Size')
plt.xscale('log')
plt.yscale('log')
plt.savefig('$OUTPUT_DIR/gas/gas_analysis.png')

# Read the network analysis data
net_df = pd.read_csv('$OUTPUT_DIR/network/network_analysis.csv')

# Create network propagation visualization
plt.figure(figsize=(12, 6))
sns.boxplot(data=net_df, y='Time Difference (ms)')
plt.title('Network Propagation Time Distribution')
plt.savefig('$OUTPUT_DIR/network/propagation_analysis.png')

# Generate summary statistics
with open('$OUTPUT_DIR/summary_statistics.txt', 'w') as f:
    f.write('Gas Usage Statistics:\n')
    f.write(gas_df.groupby('Message Size (bytes)')['Gas Used'].describe().to_string())
    f.write('\n\nNetwork Propagation Statistics:\n')
    f.write(net_df['Time Difference (ms)'].describe().to_string())
EOF
}

# Main execution
echo "Starting blockchain testing suite..."

# Run gas usage tests
echo "Testing gas usage with various message sizes..."
test_gas_usage

# Run network analysis
echo "Analyzing network propagation..."
analyze_network

# Generate visualization and reports
echo "Generating reports..."
generate_network_report

echo "Testing completed. Results available in $OUTPUT_DIR"
