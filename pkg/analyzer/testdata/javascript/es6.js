// ES6 module with import statements
import { foo, bar } from './utils';
import { Baz } from '../types';
import lodash from 'lodash';
import * as helpers from './helpers';

export function processData(data) {
  return foo(data);
}
