# Five-minute CLI demo

Use this for a soft demo to a friendly engineer before any public launch.

## Setup (once)

```bash
brew install ast-grep   # or: npm i -g @ast-grep/cli@0.45.1
git clone https://github.com/Pieczasz/tarsier.git
cd tarsier
make build
```

## Script

1. **Positive control** — show planted defects light up:

   ```bash
   ./bin/tarsier scan fixtures/_badshop
   ```

   Point at 2–3 **high**-confidence hits (cardinality label, kafka produce without inject, unstructured log). Skip narrating medium findings unless asked.

2. **HTML report** — open in a browser:

   ```bash
   ./bin/tarsier --output html scan fixtures/_badshop > /tmp/tarsier-badshop.html
   open /tmp/tarsier-badshop.html   # or xdg-open
   ```

3. **CI gate** — medium never fails the build; warnings/errors can:

   ```bash
   ./bin/tarsier scan --fail-on warning fixtures/_badshop; echo "exit=$?"
   ```

4. **Baseline** — adopt without failing day one:

   ```bash
   ./bin/tarsier scan fixtures/_badshop --baseline-write /tmp/base.json
   ./bin/tarsier scan fixtures/_badshop --baseline /tmp/base.json --fail-on warning; echo "exit=$?"
   ```

5. **Optional Go deep tier** (if time):

   ```bash
   ./bin/tarsier scan --engine=all fixtures/godeep
   ```

## Talking points

- Runs on a git checkout; no Tempo/Prometheus credentials required.
- High-confidence rules are for gates; medium is advisory.

## Soft-feedback checklist

After the demo, ask:

- [ ] Was install clear?
- [ ] Were the first three findings understandable?
- [ ] Would you run `--fail-on warning` in CI on a real service?
- [ ] What felt noisy or missing?

Capture answers in your notes (do not put customer names in this public repo).
