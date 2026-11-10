(() => {
  const visible = e => !!(e.getClientRects().length) && getComputedStyle(e).visibility !== 'hidden';
  const text = e => (e.innerText || e.textContent || '').replace(/\s+/g, ' ').trim();
  const days = ['Sunday','Monday','Tuesday','Wednesday','Thursday','Friday','Saturday'];
  const dayName = s => days.find(d => d.toLowerCase() === s.trim().toLowerCase().replace(/s$/, '')) || '';
  const result = {title: text(document.querySelector('h1') || document.createElement('span')), url: location.href, blocked: false, limited_view: /limited view of Google Maps/i.test(document.body.innerText), busy: [], hours: []};
  result.blocked = /consent\.google|\/sorry\//.test(location.href) || !!document.querySelector('form[action*="consent"],iframe[src*="recaptcha"],#captcha-form');
  if (result.blocked) return result;
  // Read only actual accessible popular-times labels, not CSS bar heights.
  // Google changes its DOM; unsupported layouts yield missing data.
  const labels = [...document.querySelectorAll('[aria-label]')].filter(visible).filter(e => !e.closest('[aria-hidden="true"]')).filter(e => {
    const label = e.getAttribute('aria-label') || '';
    return /\bbusy\b/i.test(label) && /\b(?:at\s+\d|\d{1,2}(?::\d{2})?\s*(?:am|pm))\b/i.test(label) && !/\blive\b|right now|currently/i.test(label);
  });
  const groups = new Map();
  for (const element of labels) {
    // Associate a day only if an enclosing popular-times section explicitly
    // identifies one selected weekday. Otherwise retain an unknown day.
    let day = '';
    for (let parent = element.parentElement; parent && parent !== document.body; parent = parent.parentElement) {
      if (!/Popular times/i.test(parent.innerText || '')) continue;
      const selected = [...parent.querySelectorAll('[aria-selected="true"],[role="combobox"],button[aria-haspopup],[role="button"]')]
        .filter(visible).map(e => dayName(text(e)) || dayName(e.getAttribute('aria-label') || '')).filter(Boolean);
      const unique = [...new Set(selected)];
      if (unique.length === 1) day = unique[0];
      break;
    }
    if (!groups.has(day)) groups.set(day, new Set());
    groups.get(day).add(element.getAttribute('aria-label').trim());
  }
  result.busy = [...groups].map(([day, labels]) => ({day, labels: [...labels]}));
  // Keep displayed hours verbatim, including split/overnight and special hours.
  // Limited-view Maps renders this outside role=main. The caller validates
  // the single place heading; require an explicit weekday and hours cell.
  const hours = new Map();
  // Read the primary business-hours table, including rows outside the clipped
  // viewport. Other tables may describe kitchen/drive-through hours; never
  // merge those into the business schedule or substitute them for missing days.
  for (const table of document.querySelectorAll('table,[role="table"]')) {
    const current = new Map();
    for (const row of table.querySelectorAll('tr,[role="row"]')) {
      const rawCells = [...row.querySelectorAll('td,th,[role="cell"],[role="rowheader"]')];
      const cells = rawCells.map(text);
      if (cells.length < 2) continue;
      const day = dayName(cells[0]);
      const periods = rawCells[1] ? [...rawCells[1].querySelectorAll('li')].map(text).filter(Boolean) : [];
      const value = periods.length ? periods.join(', ') : cells[1];
      if (day && value && !current.has(day)) current.set(day, value);
    }
    if (current.size) {
      for (const [day, value] of current) hours.set(day, value);
      break;
    }
  }
  result.hours = [...hours].map(([day, text]) => ({day, text}));
  return result;
})()
