use std::ops::Bound;

/// A bound on time for filtering.
#[derive(uniffi::Enum, Debug, Clone, Copy)]
pub enum TimeBound {
    Unbounded,
    Included(u64),
    Excluded(u64),
}

impl From<Bound<u64>> for TimeBound {
    fn from(b: Bound<u64>) -> Self {
        match b {
            Bound::Unbounded => TimeBound::Unbounded,
            Bound::Included(t) => TimeBound::Included(t),
            Bound::Excluded(t) => TimeBound::Excluded(t),
        }
    }
}

impl From<TimeBound> for Bound<u64> {
    fn from(b: TimeBound) -> Self {
        match b {
            TimeBound::Unbounded => Bound::Unbounded,
            TimeBound::Included(t) => Bound::Included(t),
            TimeBound::Excluded(t) => Bound::Excluded(t),
        }
    }
}
