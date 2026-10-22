"""Compatibility entry point for the documented local startup."""
from pathlib import Path
import subprocess
subprocess.run(['sh', 'scripts/start.sh'], cwd=Path(__file__).resolve().parent.parent, check=True)
