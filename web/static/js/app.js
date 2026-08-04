// Every form marked data-json submits its fields as a JSON body instead of
// the browser's default application/x-www-form-urlencoded, matching what
// every backend handler here (json.NewDecoder(r.Body).Decode) expects. No
// X-Client-Type header is set, so the server defaults to "web" and returns
// plain HTML (a redirect for auth actions, an HTML fragment otherwise) —
// exactly what a browser fetch() call wants back.
//
// fetch() follows redirects transparently: after a 303, response.redirected
// is true and response.url is the final page, so we navigate there for a
// real full-page load. When the response isn't a redirect (a fragment or an
// error message), its HTML is swapped into the form's data-target element.
document.addEventListener('submit', function (event) {
  var form = event.target;
  if (!(form instanceof HTMLFormElement) || !form.matches('form[data-json]')) {
    return;
  }
  event.preventDefault();

  var targetSelector = form.getAttribute('data-target');
  var target = targetSelector ? document.querySelector(targetSelector) : null;
  var submitButton = form.querySelector('button[type="submit"]');

  // Built from form.elements rather than FormData, since the backoffice
  // forms (M12) need type-aware conversion FormData can't do:
  //   - checkbox -> real JSON boolean (FormData omits unchecked boxes
  //     entirely and gives checked ones the string "on", neither of which
  //     json.Decode into a Go *bool)
  //   - number -> real JSON number (a JSON string like "40.5" fails to
  //     decode into a Go float64 field without a `json:",string"` tag)
  //   - datetime-local -> a full RFC3339 string (its native value, e.g.
  //     "2024-01-15T14:30", has no seconds/timezone and isn't valid
  //     RFC3339, which is what Go's time.Time expects)
  //   - blank optional fields are omitted entirely rather than sent as ""
  //     (json.Decode of "" into a Go *T pointer/slice field fails the same
  //     way); required fields are guaranteed non-blank by native HTML5
  //     validation before submit ever runs.
  var data = {};
  Array.prototype.forEach.call(form.elements, function (el) {
    if (!el.name) return;
    if (el.type === 'checkbox') {
      data[el.name] = el.checked;
      return;
    }
    if (el.type === 'radio') {
      if (el.checked) data[el.name] = el.value;
      return;
    }
    if (el.value === '' && !el.required) return;
    if (el.type === 'number') {
      data[el.name] = Number(el.value);
    } else if (el.type === 'datetime-local') {
      data[el.name] = new Date(el.value).toISOString();
    } else {
      data[el.name] = el.value;
    }
  });

  if (submitButton) submitButton.disabled = true;

  fetch(form.getAttribute('action'), {
    method: form.getAttribute('method') || 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
    .then(function (response) {
      if (response.redirected) {
        window.location.href = response.url;
        return null;
      }
      return response.text();
    })
    .then(function (html) {
      if (html !== null && target) target.innerHTML = html;
    })
    .catch(function () {
      if (target) target.innerHTML = '<p class="error">Network error, please try again.</p>';
    })
    .finally(function () {
      if (submitButton) submitButton.disabled = false;
    });
});
