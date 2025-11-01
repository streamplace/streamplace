#[derive(Debug, thiserror::Error, uniffi::Error)]
#[uniffi(flat_error)]
pub enum SPError {
    #[error("No certificate chain found")]
    NoCertificateChainFound,
    #[error("C2PA error: {0}")]
    C2paError(String),
    #[error("IO Error: {0}")]
    IOError(String),
}
