// TypeScript module with type imports
import { foo } from './utils';
import type { Baz } from './interfaces';
import { Component } from 'react';
import * as helpers from '../helpers';

interface Data {
  value: string;
}

export function processData(data: Data): string {
  return foo(data);
}
