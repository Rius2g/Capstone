import React, { useEffect } from 'react';
import socket from './websocket';
import { DataDisplay } from './DataDisplay';
import { useAuth0 } from '@auth0/auth0-react';

export const MainPage = () => {
    const [data, setData] = React.useState<string[]>([]);
    const { loginWithRedirect, logout, isAuthenticated, user } = useAuth0();
    const [blockNumber, setBlockNumber] = React.useState<number>(0);

    // React.useEffect(() => {
    //     socket.addEventListener('message', (event) => {
    //         console.log('Message from server:', event.data);
    //         setData((prevData) => [...prevData, event.data]);
    //     });
    // })
    React.useEffect(() => {
        document.title = 'Main Page';
        fetch('http://localhost:8080/blockNumber')
        .then(response => {
            if (!response.ok) {
                throw new Error('Network response was not ok');
            }
            return response.json();
        })
        .then(data => {
            console.log(data.blockNumber);
            setBlockNumber(data.blockNumber);
    })
        .catch(error => console.error('Fetch error:', error));
    
    }
    , []);


    return ( 
           <div>
           <h1> Heia verden </h1>
            <h2>INSERT LIST OF PENDING BROADCASTS OR REAL TIME DATASTREAM</h2>
            <div>
                {isAuthenticated && <span>Welcome, {user?.name || 'User'}</span>}
            </div>
            <h2>BlockNumber: {blockNumber}</h2>
            <DataDisplay data={data} />
        </div>
        )

}
