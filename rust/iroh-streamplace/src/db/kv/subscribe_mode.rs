use iroh_smol_kv as w;

/// Subscription mode for key-value subscriptions.
#[derive(uniffi::Enum, Debug, Clone, Copy)]
pub enum SubscribeMode {
    Current,
    Future,
    Both,
}

impl From<w::SubscribeMode> for SubscribeMode {
    fn from(m: w::SubscribeMode) -> Self {
        match m {
            w::SubscribeMode::Current => SubscribeMode::Current,
            w::SubscribeMode::Future => SubscribeMode::Future,
            w::SubscribeMode::Both => SubscribeMode::Both,
        }
    }
}

impl From<SubscribeMode> for w::SubscribeMode {
    fn from(m: SubscribeMode) -> Self {
        match m {
            SubscribeMode::Current => w::SubscribeMode::Current,
            SubscribeMode::Future => w::SubscribeMode::Future,
            SubscribeMode::Both => w::SubscribeMode::Both,
        }
    }
}
