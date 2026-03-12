# Python file with relative imports
# Used for testing parsePythonFiles

from . import simple  # relative import

def greet():
    return simple.hello()

if __name__ == "__main__":
    print(greet())
