#!/usr/bin/env python3
"""Derive the original-brand terms from required credits/notices, then scan shipping files."""
import json,re,subprocess,base64
from pathlib import Path
root=Path(__file__).resolve().parent.parent
credit=re.search(r'Built on \[([^]]+)\]',(root/'README.md').read_text()).group(1)
terms={part.strip() for part in credit.split('/')}
paths=sorted(set(p.decode() for p in subprocess.check_output(['git','ls-files','--cached','--others','--exclude-standard','-z'],cwd=root).split(b'\0') if p))
paths=[p for p in paths if (root/p).is_file()]
for name in paths:
 if name in {'LICENSE-ORIGINAL','NOTICE','web/NOTICE','site/NOTICE'}:
  for owner in re.findall(r'^\s*Copyright(?:\s+\(c\))?\s+\d{4}\s+(.+)$',(root/name).read_text(errors='replace'),re.M):terms.add(owner.strip());terms.add(owner.split()[0])
checked=[p for p in paths if p!='README.md' and Path(p).name not in {'NOTICE','LICENSE-ORIGINAL'}]
pattern='|'.join(re.escape(term) for term in sorted(terms) if term)
r=subprocess.run(['rg','--json','--hidden','-i',pattern,'--',*checked],cwd=root,text=True,capture_output=True)
matches=[json.loads(line)['data'] for line in r.stdout.splitlines() if json.loads(line)['type']=='match']
print(json.dumps({'source':'required README credit and original copyright notices','matching_lines':len(matches),'matching_files':sorted(set(d['path'].get('text','') for d in matches))}))
