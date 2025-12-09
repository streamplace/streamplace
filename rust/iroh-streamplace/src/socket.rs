use std::sync::Arc;

use iroh_base::ticket::NodeTicket;

use super::RUNTIME;

/// A wrapper for an iroh endpoint that works basically as a socket for streams.
#[derive(Debug, uniffi::Object)]
pub struct Socket {
    endpoint: iroh::Endpoint,
    alpn: Vec<u8>,
}

/// Configuration for creating a [`Socket`].
#[derive(Debug, uniffi::Record)]
pub struct SocketConfig {
    /// A 32-byte secret key for the socket.
    pub secret: Vec<u8>,
    /// The ALPN to use for this socket.
    pub alpn: Vec<u8>,
}

#[uniffi::export]
impl Socket {

    /// Create a new [`Socket`] with the given [`SocketConfig`].
    #[uniffi::constructor]
    pub async fn new(config: SocketConfig) -> Result<Self, SocketNewError> {
        RUNTIME.block_on(Self::new0(config))
    }

    /// Wait until the socket is online.
    pub async fn online(&self) {
        RUNTIME.block_on(self.endpoint.online());
    }

    /// Get the ticket for this socket.
    pub fn ticket(&self) -> String {
        let addr = self.endpoint.node_addr();
        let ticket = NodeTicket::from(addr);
        ticket.to_string()
    }

    /// Get the ALPN for this socket.
    pub fn alpn(&self) -> Vec<u8> {
        self.alpn.clone()
    }

    /// Accept an incoming connection and return a [`Stream2`].
    pub async fn accept(&self) -> Result<Arc<Stream2>, AcceptError> {
        RUNTIME.block_on(self.accept0())
    }

    /// Connect to a peer at the given [`NodeAddr`] and return a [`Stream2`].
    pub async fn connect(
        &self,
        addr: Arc<crate::node_addr::NodeAddr>,
    ) -> Result<Arc<Stream2>, ConnectError> {
        RUNTIME.block_on(self.connect0(addr))
    }

    /// Close the socket.
    pub async fn close(&self) {
        RUNTIME.block_on(self.endpoint.close())
    }
}

impl Socket {

    async fn new0(config: SocketConfig) -> Result<Self, SocketNewError> {
        let secret_bytes: &[u8; 32] = config.secret.as_slice().try_into().map_err(|_| SocketNewError::Other {
            message: "Invalid secret key length".to_string(),
        })?;
        let secret_key = iroh::SecretKey::from_bytes(secret_bytes);
        let endpoint = iroh::Endpoint::builder().secret_key(secret_key)
        .alpns(vec![config.alpn.clone()])
        .bind().await.map_err(|e| SocketNewError::Other {
            message: e.to_string(),
        })?;
        Ok(Socket {
            endpoint,
            alpn: config.alpn.clone(),
        })
    }

    async fn accept0(&self) -> Result<Arc<Stream2>, AcceptError> {
        let incoming = self
            .endpoint
            .accept()
            .await
            .ok_or_else(|| AcceptError::Other {
                message: "Failed to accept connection".to_string(),
            })?;
        let conn = incoming.await.map_err(|e| AcceptError::Other {
            message: e.to_string(),
        })?;
        let (send, recv) = conn.accept_bi().await.map_err(|e| AcceptError::Other {
            message: e.to_string(),
        })?;
        Ok(Arc::new(Stream2 {
            recv: tokio::sync::Mutex::new(recv),
            send: tokio::sync::Mutex::new(send),
            conn,
        }))
    }

    async fn connect0(
        &self,
        addr: Arc<crate::node_addr::NodeAddr>,
    ) -> Result<Arc<Stream2>, ConnectError> {
        let node_addr: iroh::NodeAddr =
            (*addr)
                .clone()
                .try_into()
                .map_err(|_| ConnectError::Other {
                    message: "Invalid node address".to_string(),
                })?;
        let conn = self
            .endpoint
            .connect(node_addr, &self.alpn)
            .await
            .map_err(|e| ConnectError::Other {
                message: e.to_string(),
            })?;
        let (send, recv) = conn.open_bi().await.map_err(|e| ConnectError::Other {
            message: e.to_string(),
        })?;
        Ok(Arc::new(Stream2 {
            recv: tokio::sync::Mutex::new(recv),
            send: tokio::sync::Mutex::new(send),
            conn,
        }))
    }
}

#[derive(Debug, thiserror::Error, uniffi::Error)]
#[uniffi(flat_error)]
pub enum AcceptError {
    #[error("Other error: {message}")]
    Other { message: String },
}

#[derive(Debug, thiserror::Error, uniffi::Error)]
#[uniffi(flat_error)]
pub enum ConnectError {
    #[error("Other error: {message}")]
    Other { message: String },
}

#[derive(Debug, thiserror::Error, uniffi::Error)]
#[uniffi(flat_error)]
pub enum ReadError {
    #[error("Other error: {message}")]
    Other { message: String },
}

#[derive(Debug, thiserror::Error, uniffi::Error)]
#[uniffi(flat_error)]
pub enum WriteError2 {
    #[error("Other error: {message}")]
    Other { message: String },
}

#[derive(Debug, thiserror::Error, uniffi::Error)]
#[uniffi(flat_error)]
pub enum SocketNewError {
    #[error("Other error: {message}")]
    Other { message: String },
}

#[derive(Debug, uniffi::Object)]
pub struct Stream2 {
    recv: tokio::sync::Mutex<iroh::endpoint::RecvStream>,
    send: tokio::sync::Mutex<iroh::endpoint::SendStream>,
    conn: iroh::endpoint::Connection,
}

#[uniffi::export]
impl Stream2 {
    pub async fn read(&self, n: u64) -> Result<Vec<u8>, ReadError> {
        let mut buf = vec![0u8; n as usize];
        let n = self
            .recv
            .lock()
            .await
            .read(&mut buf)
            .await
            .map_err(|e| ReadError::Other {
                message: e.to_string(),
            })?
            .ok_or_else(|| ReadError::Other {
                message: "connection closed".to_string(),
            })?;
        buf.truncate(n);
        Ok(buf)
    }

    pub async fn write_all(&self, buf: &[u8]) -> Result<(), WriteError2> {
        self.send.lock().await.write_all(buf).await.map_err(|e| WriteError2::Other {
            message: e.to_string(),
        })
    }

    pub async fn write(&self, buf: &[u8]) -> Result<u32, WriteError2> {
        let n = self.send
            .lock()
            .await
            .write(&buf)
            .await
            .map_err(|e| WriteError2::Other {
                message: e.to_string(),
            })?;
        Ok(n as u32)
    }

    pub async fn close_write(&self) -> Result<(), WriteError2> {
        self.send.lock().await.finish().map_err(|e| WriteError2::Other {
            message: e.to_string(),
        })?;
        Ok(())
    }

    pub async fn close_read(&self) -> Result<(), ReadError> {
        self.recv.lock().await.stop(0u32.into()).map_err(|e| ReadError::Other {
            message: e.to_string(),
        })?;
        Ok(())
    }

    pub fn close(&self) {
        self.conn.close(0u32.into(), b"");
    }

    pub async fn closed(&self) {
        RUNTIME.block_on(async {
            let _reason = self.conn.closed().await;
        });
    }
}
