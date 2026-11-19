// berAPI wireframe — interaksi dasar (tabs, wizard, tree, toast)
document.addEventListener('DOMContentLoaded', () => {
  // tabs: elemen [data-tabs] berisi tombol data-tab -> panel #<tab>
  document.querySelectorAll('[data-tabs]').forEach((bar) => {
    const btns = bar.querySelectorAll('button[data-tab]');
    btns.forEach((b) => b.addEventListener('click', () => {
      btns.forEach((x) => x.classList.remove('active'));
      b.classList.add('active');
      const scope = bar.closest('[data-scope]') || document;
      scope.querySelectorAll('.tabpane').forEach((p) => p.classList.remove('active'));
      const pane = scope.querySelector('#' + b.dataset.tab);
      if (pane) pane.classList.add('active');
    }));
  });

  // wizard: [data-wizard] dengan [data-step] dan tombol data-next/data-prev
  document.querySelectorAll('[data-wizard]').forEach((w) => {
    const steps = w.querySelectorAll('[data-step]');
    const dots = w.querySelectorAll('[data-dot]');
    let idx = 0;
    const render = () => {
      steps.forEach((s, i) => s.style.display = i === idx ? 'block' : 'none');
      dots.forEach((d, i) => d.classList.toggle('active', i === idx));
    };
    w.querySelectorAll('[data-next]').forEach((b) => b.addEventListener('click', () => { idx = Math.min(idx + 1, steps.length - 1); render(); }));
    w.querySelectorAll('[data-prev]').forEach((b) => b.addEventListener('click', () => { idx = Math.max(idx - 1, 0); render(); }));
    render();
  });

  // toast
  window.toast = (msg) => {
    const t = document.querySelector('.toast');
    if (!t) return;
    t.textContent = msg;
    t.classList.add('show');
    setTimeout(() => t.classList.remove('show'), 2200);
  };
  document.querySelectorAll('[data-toast]').forEach((b) =>
    b.addEventListener('click', () => toast(b.dataset.toast)));
});
