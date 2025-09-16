use std::str::FromStr;
use std::sync::Arc;

use crate::error::Error;
use crate::key::PublicKey;

/// A peer and it's addressing information.
#[derive(Debug, Clone, PartialEq, Eq, uniffi::Object)]
pub struct NodeAddr {
    node_id: Arc<PublicKey>,
    relay_url: Option<String>,
    addresses: Vec<String>,
}

#[uniffi::export]
impl NodeAddr {
    /// Create a new [`NodeAddr`] with empty [`AddrInfo`].
    #[uniffi::constructor]
    pub fn new(node_id: &PublicKey, derp_url: Option<String>, addresses: Vec<String>) -> Self {
        Self {
            node_id: Arc::new(node_id.clone()),
            relay_url: derp_url,
            addresses,
        }
    }

    pub fn node_id(&self) -> PublicKey {
        self.node_id.as_ref().clone()
    }

    /// Get the direct addresses of this peer.
    pub fn direct_addresses(&self) -> Vec<String> {
        self.addresses.clone()
    }

    /// Get the home relay URL for this peer
    pub fn relay_url(&self) -> Option<String> {
        self.relay_url.clone()
    }

    /// Returns true if both NodeAddr's have the same values
    pub fn equal(&self, other: &NodeAddr) -> bool {
        self == other
    }
}

impl TryFrom<NodeAddr> for iroh::NodeAddr {
    type Error = Error;

    fn try_from(value: NodeAddr) -> Result<Self, Self::Error> {
        let mut node_addr = iroh::NodeAddr::new((&*value.node_id).into());
        let addresses = value
            .direct_addresses()
            .into_iter()
            .map(|addr| std::net::SocketAddr::from_str(&addr).map_err(Error::from))
            .collect::<Result<Vec<_>, _>>()?;

        if let Some(derp_url) = value.relay_url() {
            let url = url::Url::parse(&derp_url)?;

            node_addr = node_addr.with_relay_url(url.into());
        }
        node_addr = node_addr.with_direct_addresses(addresses);
        Ok(node_addr)
    }
}

impl From<iroh::NodeAddr> for NodeAddr {
    fn from(value: iroh::NodeAddr) -> Self {
        NodeAddr {
            node_id: Arc::new(value.node_id.into()),
            relay_url: value.relay_url.map(|url| url.to_string()),
            addresses: value
                .direct_addresses
                .into_iter()
                .map(|d| d.to_string())
                .collect(),
        }
    }
}
