// Copyright 2023 Adobe. All rights reserved.
// This file is licensed to you under the Apache License,
// Version 2.0 (http://www.apache.org/licenses/LICENSE-2.0)
// or the MIT license (http://opensource.org/licenses/MIT),
// at your option.

// Unless required by applicable law or agreed to in writing,
// this software is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR REPRESENTATIONS OF ANY KIND, either express or
// implied. See the LICENSE-MIT and LICENSE-APACHE files for the
// specific language governing permissions and limitations under
// each license.

use crate::error::SPError;
use std::io::{Read, Seek, SeekFrom, Write};
use std::sync::Arc;

// #[repr(C)]
// #[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
// pub enum SeekMode {
//     Start = 0,
//     End = 1,
//     Current = 2,
// }

/// This allows for a callback stream over the Uniffi interface.
/// Implement these stream functions in the foreign language
/// and this will provide Rust Stream trait implementations
/// This is necessary since the Rust traits cannot be implemented directly
/// as uniffi callbacks
#[uniffi::export(with_foreign)]
pub trait Stream: Send + Sync {
    /// Read a stream of bytes from the stream
    fn read_stream(&self, length: u64) -> Result<Vec<u8>, SPError>;
    /// Seek to a position in the stream
    fn seek_stream(&self, pos: i64, mode: u64) -> Result<u64, SPError>;
    /// Write a stream of bytes to the stream
    fn write_stream(&self, data: Vec<u8>) -> Result<u64, SPError>;
}

impl Stream for Arc<dyn Stream> {
    fn read_stream(&self, length: u64) -> Result<Vec<u8>, SPError> {
        (**self).read_stream(length)
    }

    fn seek_stream(&self, pos: i64, mode: u64) -> Result<u64, SPError> {
        (**self).seek_stream(pos, mode)
    }

    fn write_stream(&self, data: Vec<u8>) -> Result<u64, SPError> {
        (**self).write_stream(data)
    }
}

impl AsMut<dyn Stream> for dyn Stream {
    fn as_mut(&mut self) -> &mut Self {
        self
    }
}

pub struct StreamAdapter<'a> {
    pub stream: &'a mut dyn Stream,
}

impl<'a> StreamAdapter<'a> {
    pub fn from_stream_mut(stream: &'a mut dyn Stream) -> Self {
        Self { stream }
    }
}

impl<'a> From<&'a dyn Stream> for StreamAdapter<'a> {
    #[allow(invalid_reference_casting)]
    fn from(stream: &'a dyn Stream) -> Self {
        let stream = &*stream as *const dyn Stream as *mut dyn Stream;
        let stream = unsafe { &mut *stream };
        Self { stream }
    }
}

// impl<'a> c2pa::CAIRead for StreamAdapter<'a> {}

// impl<'a> c2pa::CAIReadWrite for StreamAdapter<'a> {}

impl<'a> Read for StreamAdapter<'a> {
    fn read(&mut self, buf: &mut [u8]) -> std::io::Result<usize> {
        let mut bytes = self
            .stream
            .read_stream(buf.len() as u64)
            .map_err(|e| std::io::Error::new(std::io::ErrorKind::Other, e))?;
        let len = bytes.len();
        buf.iter_mut().zip(bytes.drain(..)).for_each(|(dest, src)| {
            *dest = src;
        });
        //println!("read: {:?}", len);
        Ok(len)
    }
}

impl<'a> Seek for StreamAdapter<'a> {
    fn seek(&mut self, pos: std::io::SeekFrom) -> std::io::Result<u64> {
        let (pos, mode) = match pos {
            SeekFrom::Current(pos) => (pos, 2),
            SeekFrom::Start(pos) => (pos as i64, 0),
            SeekFrom::End(pos) => (pos, 1),
        };
        //println!("Stream Seek {}", pos);
        self.stream
            .seek_stream(pos, mode)
            .map_err(|e| std::io::Error::new(std::io::ErrorKind::Other, e))
    }
}

impl<'a> Write for StreamAdapter<'a> {
    fn write(&mut self, buf: &[u8]) -> std::io::Result<usize> {
        let len = self
            .stream
            .write_stream(buf.to_vec())
            .map_err(|e| std::io::Error::new(std::io::ErrorKind::Other, e))?;
        Ok(len as usize)
    }

    fn flush(&mut self) -> std::io::Result<()> {
        Ok(())
    }
}

#[uniffi::export(with_foreign)]
pub trait ManyStreams: Send + Sync {
    /// Get the next stream from the many streams
    fn next(&self) -> Result<Option<Arc<dyn Stream>>, SPError>;
}

#[cfg(test)]
mod tests {
    use super::*;

    use crate::test_stream::TestStream;

    #[test]
    fn test_stream_read() {
        let mut test = TestStream::from_memory(vec![0, 1, 2, 3, 4, 5, 6, 7, 8, 9]);
        let mut stream = StreamAdapter::from_stream_mut(&mut test);
        let mut buf = [0u8; 5];
        let len = stream.read(&mut buf).unwrap();
        assert_eq!(len, 5);
        assert_eq!(buf, [0, 1, 2, 3, 4]);
    }

    #[test]
    fn test_stream_seek() {
        let mut test = TestStream::from_memory(vec![0, 1, 2, 3, 4, 5, 6, 7, 8, 9]);
        let mut stream = StreamAdapter { stream: &mut test };
        let pos = stream.seek(SeekFrom::Start(5)).unwrap();
        assert_eq!(pos, 5);
        let mut buf = [0u8; 5];
        let len = stream.read(&mut buf).unwrap();
        assert_eq!(len, 5);
        assert_eq!(buf, [5, 6, 7, 8, 9]);
    }

    #[test]
    fn test_stream_write() {
        let mut test = TestStream::new();
        let mut stream = StreamAdapter { stream: &mut test };
        let len = stream.write(&[0, 1, 2, 3, 4]).unwrap();
        assert_eq!(len, 5);
        stream.seek(SeekFrom::Start(0)).unwrap();
        let mut buf = [0u8; 5];
        let len = stream.read(&mut buf).unwrap();
        assert_eq!(len, 5);
        assert_eq!(buf, [0, 1, 2, 3, 4]);
    }
}
