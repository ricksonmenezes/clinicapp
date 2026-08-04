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

  var data = {};
  new FormData(form).forEach(function (value, key) {
    data[key] = value;
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
