# Python file with absolute imports
# Used for testing parsePythonFiles

import internal.utils
import json  # stdlib - should be filtered

def process_data(data):
    return internal.utils.transform(data)

if __name__ == "__main__":
    data = json.loads('{"key": "value"}')
    print(process_data(data))
