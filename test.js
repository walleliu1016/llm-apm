#!/usr/bin/env node
/*
 * Socket.IO end-to-end smoke test for Ease Island Cloud Server.
 *
 * Prerequisites:
 *   1. Start Cloud Server on port 17528.
 *   2. Ensure TEST_USER_ID is in whitelist when ENABLE_WHITELIST=true.
 *   3. Install socket.io-client where this script runs:
 *        npm install socket.io-client@^4.7.0
 *
 * Usage:
 *   CLOUD_URL=http://localhost:17528 TEST_USER_ID=test-user node test/socketio-e2e-test.js
 */

let io
try {
  ;({ io } = require('socket.io-client'))
} catch (err) {
  console.error('Missing dependency: socket.io-client')
  console.error('Install it with: npm install socket.io-client@^4.7.0')
  process.exit(1)
}

const CLOUD_URL = process.env.CLOUD_URL || 'http://localhost:17528'
const TEST_USER_ID = process.env.TEST_USER_ID || 'test-user'
const TEST_DESKTOP_ID = process.env.TEST_DESKTOP_ID || 'test-desktop'
const TEST_SESSION_ID = process.env.TEST_SESSION_ID || `test-session-${Date.now()}`
const TIMEOUT_MS = Number(process.env.TEST_TIMEOUT_MS || 15000)

function waitFor(socket, event, timeoutMs = TIMEOUT_MS) {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      cleanup()
      reject(new Error(`Timed out waiting for ${event}`))
    }, timeoutMs)

    function cleanup() {
      clearTimeout(timer)
      socket.off(event, onEvent)
      socket.off('connect_error', onError)
      socket.off('auth:failed', onAuthFailed)
    }

    function onEvent(data) {
      cleanup()
      resolve(data)
    }

    function onError(err) {
      cleanup()
      reject(new Error(`connect_error: ${err.message}`))
    }

    function onAuthFailed(data) {
      cleanup()
      reject(new Error(`auth:failed: ${data && data.reason ? data.reason : 'unknown'}`))
    }

    socket.once(event, onEvent)
    socket.once('connect_error', onError)
    socket.once('auth:failed', onAuthFailed)
  })
}

function createSocket(label) {
  const socket = io(CLOUD_URL, {
    reconnection: false,
    transports: ['websocket', 'polling'],
    timeout: TIMEOUT_MS,
  })

  socket.on('connect', () => console.log(`[${label}] connected: ${socket.id}`))
  socket.on('disconnect', (reason) => console.log(`[${label}] disconnected: ${reason}`))
  socket.onAny((event, data) => {
    if (!['connect', 'disconnect'].includes(event)) {
      console.log(`[${label}] <- ${event}`, JSON.stringify(data || {}))
    }
  })

  return socket
}

async function run() {
  console.log(`Cloud URL: ${CLOUD_URL}`)
  console.log(`Test user_id: ${TEST_USER_ID}`)
  console.log(`Test session_id: ${TEST_SESSION_ID}`)

  const desktop = createSocket('desktop')
  const mobile = createSocket('mobile')

  try {
    await Promise.all([waitFor(desktop, 'connect'), waitFor(mobile, 'connect')])

    desktop.emit('desktop:auth', {
      desktop_id: TEST_DESKTOP_ID,
      hostname: 'socketio-e2e-test-host',
    })
    console.log('[desktop] -> desktop:auth')
    await waitFor(desktop, 'auth:success')
    console.log('[desktop] authenticated')

    mobile.emit('mobile:auth', {
      user_id: TEST_USER_ID,
    })
    console.log('[mobile] -> mobile:auth')
    await waitFor(mobile, 'auth:success')
    console.log('[mobile] authenticated')

    const hookPromise = waitFor(mobile, 'hook:message')
    desktop.emit('hook:message', {
      user_id: TEST_USER_ID,
      session_id: TEST_SESSION_ID,
      hook_type: 'SessionStart',
      hook_body: {
        hook_event_name: 'SessionStart',
        session_id: TEST_SESSION_ID,
        cwd: '/tmp/socketio-e2e-test',
        project_name: 'socketio-e2e-test',
      },
    })
    console.log('[desktop] -> hook:message')

    const hook = await hookPromise
    if (!hook || hook.session_id !== TEST_SESSION_ID || hook.user_id !== TEST_USER_ID) {
      throw new Error(`Unexpected hook payload: ${JSON.stringify(hook)}`)
    }
    console.log('[mobile] received expected hook:message')

    const responsePromise = waitFor(desktop, 'hook:response')
    mobile.emit('hook:response', {
      user_id: TEST_USER_ID,
      session_id: TEST_SESSION_ID,
      decision: 'allow',
    })
    console.log('[mobile] -> hook:response')

    const response = await responsePromise
    if (!response || response.session_id !== TEST_SESSION_ID || response.user_id !== TEST_USER_ID) {
      throw new Error(`Unexpected response payload: ${JSON.stringify(response)}`)
    }
    console.log('[desktop] received expected hook:response')

    console.log('Socket.IO E2E smoke test passed')
  } finally {
    desktop.close()
    mobile.close()
  }
}

run().catch((err) => {
  console.error('Socket.IO E2E smoke test failed:', err.message)
  process.exit(1)
})
