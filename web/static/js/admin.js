// Backoffice-only helpers layered on top of app.js (form submission) and
// htmx (list fragments/hx-get). Two things neither of those handle out of
// the box:
//
// 1) Populating an edit form from a list click. The existing web-mode HTML
//    fragments (e.g. patientHTML, consultantHTML) only carry an id and one
//    or two display fields — enough for a list, not enough to pre-fill an
//    edit form. So a click fetches the full JSON record instead, using
//    X-Client-Type: mobile (a header, not a route) to get the JSON shape
//    every existing handler already serves for mobile clients.
//
// 2) The consultant "per-service commission override" sub-resource, whose
//    URL is parameterized by whichever consultant is currently selected —
//    hx-get's URL is static, so this one dynamic case uses htmx's own
//    JS API (htmx.ajax) to reuse the existing HTML fragment instead of
//    hand-rolling a client-side renderer.
document.addEventListener('click', function (event) {
  var li = event.target.closest('li[data-id]');
  if (!li) return;
  var section = li.closest('[data-admin-resource]');
  if (!section) return;

  var resource = section.getAttribute('data-admin-resource');
  var id = li.getAttribute('data-id');

  var form = section.querySelector('form.admin-edit-form');
  if (form) {
    fetch('/' + resource + '/' + id, { headers: { 'X-Client-Type': 'mobile' } })
      .then(function (r) { return r.json(); })
      .then(function (data) {
        form.hidden = false;
        form.setAttribute('action', '/' + resource + '/' + id);
        Array.prototype.forEach.call(form.querySelectorAll('[data-field]'), function (el) {
          var key = el.getAttribute('data-field');
          if (!(key in data) || data[key] === null || data[key] === undefined) return;
          if (el.type === 'checkbox') {
            el.checked = !!data[key];
          } else {
            el.value = data[key];
          }
        });
        form.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
      });
  }

  if (resource === 'consultants') {
    var commissionSection = document.getElementById('commission-overrides');
    if (commissionSection) {
      commissionSection.hidden = false;
      var label = commissionSection.querySelector('[data-consultant-label]');
      if (label) label.textContent = id;
      var overrideForm = commissionSection.querySelector('form');
      overrideForm.setAttribute('action', '/consultants/' + id + '/service-commissions');
      var list = commissionSection.querySelector('.admin-list');
      htmx.ajax('GET', '/consultants/' + id + '/service-commissions', { target: list, swap: 'innerHTML' });
    }
  }
});

// Some create forms reference another resource by id (a session's
// service_id/consultant_id, a package's service_id, a patient-package's
// package_id...). Rather than make the admin paste raw UUIDs, any
// <select data-options-from="/services"> is populated from that
// endpoint's existing JSON list response — data-options-key names the
// envelope key (defaults to the last path segment), data-options-label
// the field to display (defaults to "name").
function populateSelect(select) {
  var url = select.getAttribute('data-options-from');
  var itemsKey = select.getAttribute('data-options-key') || url.split('/').pop();
  var labelField = select.getAttribute('data-options-label') || 'name';
  var placeholder = select.hasAttribute('required') ? '' : '<option value="">(none)</option>';

  fetch(url, { headers: { 'X-Client-Type': 'mobile' } })
    .then(function (r) { return r.json(); })
    .then(function (data) {
      var items = data[itemsKey] || [];
      select.innerHTML = placeholder + items.map(function (item) {
        return '<option value="' + item.id + '">' + (item[labelField] || item.id) + '</option>';
      }).join('');
    });
}

document.addEventListener('DOMContentLoaded', function () {
  Array.prototype.forEach.call(document.querySelectorAll('select[data-options-from]'), populateSelect);
});

// The invoice-placeholder form's PUT target (/invoice-template-placeholders/{key})
// has the key in the URL, not the body — data-key-param marks the input that
// drives it, data-action-base on the form is the URL prefix to append it to.
document.addEventListener('input', function (event) {
  var el = event.target;
  if (!el.matches('[data-key-param]')) return;
  var form = el.closest('form');
  var base = form.getAttribute('data-action-base');
  form.setAttribute('action', base + encodeURIComponent(el.value));
});
