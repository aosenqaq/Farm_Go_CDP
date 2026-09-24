import React from 'react'
import {createRoot} from 'react-dom/client'
import './style.css'
import App from './App'
import RemoteApp from './RemoteApp'
import { installRemoteBridge, type RemoteBridgeWindow } from './lib/remoteBridge'

const container = document.getElementById('root')

const root = createRoot(container!)
const remote = (window as RemoteBridgeWindow).__FARM_GO_REMOTE__ === true

installRemoteBridge()

root.render(
    <React.StrictMode>
        {remote ? <RemoteApp/> : <App/>}
    </React.StrictMode>
)
