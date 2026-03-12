// Rust file with various import types
use std::collections::HashMap;
use std::vec::Vec;
use crate::simple;
use super::other;
use self::nested;

pub fn process_data() {
    let mut map = HashMap::new();
    map.insert("key", "value");

    simple::add(1, 2);
}

mod nested {
    pub fn helper() {}
}
