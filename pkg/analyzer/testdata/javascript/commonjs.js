// CommonJS module with require statements
const utils = require('./utils');
const types = require('../types');
const express = require('express');
const helpers = require('./helpers');

module.exports = {
  processData: function(data) {
    return utils.foo(data);
  }
};
