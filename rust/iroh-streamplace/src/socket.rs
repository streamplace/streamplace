use std::sync::Arc;

use super::RUNTIME;

/// A wrapper for an iroh endpoint that works basically as a socket for streams.
#[derive(Debug, uniffi::Object)]
pub struct Socket {
    endpoint: iroh::Endpoint,
    alpn: Vec<u8>,
}

impl Socket {
    pub fn alpn(&self) -> Vec<u8> {
        self.alpn.clone()
    }

    pub async fn accept(&self) -> Result<Arc<Stream2>, AcceptError> {
        RUNTIME.block_on(self.accept0())
    }

    pub async fn connect(
        &self,
        addr: Arc<crate::node_addr::NodeAddr>,
    ) -> Result<Arc<Stream2>, ConnectError> {
        RUNTIME.block_on(self.connect0(addr))
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

#[derive(Debug, uniffi::Object)]
pub struct Stream2 {
    recv: tokio::sync::Mutex<iroh::endpoint::RecvStream>,
    send: tokio::sync::Mutex<iroh::endpoint::SendStream>,
}

impl Stream2 {
    pub async fn read(&self, n: usize) -> Result<Vec<u8>, ReadError> {
        let mut buf = vec![0u8; n];
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

    pub async fn write_all(&self, buf: Vec<u8>) -> Result<(), WriteError2> {
        self.send
            .lock()
            .await
            .write_all(&buf)
            .await
            .map_err(|e| WriteError2::Other {
                message: e.to_string(),
            })?;
        Ok(())
    }
}
