// Booking page interactivity: selecting a service and a date drives a fresh
// GET /availability?service_id=&date= fetch (plain fetch, not hx-get, since
// the params change dynamically from two independent inputs); selecting an
// available slot fills the hidden booking form fields and enables submit.
// The actual booking POST is handled by app.js's generic data-json handler.
document.addEventListener('DOMContentLoaded', function () {
  var serviceList = document.getElementById('service-list');
  var dateInput = document.getElementById('booking-date');
  var slotList = document.getElementById('slot-list');
  var selectedServiceInput = document.getElementById('selected-service-id');
  var selectedSlotInput = document.getElementById('selected-slot');
  var confirmButton = document.getElementById('confirm-booking');

  function resetSlotSelection() {
    selectedSlotInput.value = '';
    confirmButton.disabled = true;
  }

  function loadSlots() {
    resetSlotSelection();
    if (!selectedServiceInput.value || !dateInput.value) {
      slotList.innerHTML = '';
      return;
    }
    slotList.innerHTML = 'Loading available times…';
    var url = '/availability?service_id=' + encodeURIComponent(selectedServiceInput.value) +
      '&date=' + encodeURIComponent(dateInput.value);
    fetch(url)
      .then(function (response) { return response.text(); })
      .then(function (html) { slotList.innerHTML = html; })
      .catch(function () {
        slotList.innerHTML = '<p class="error">Could not load availability, please try again.</p>';
      });
  }

  serviceList.addEventListener('click', function (event) {
    var item = event.target.closest('li[data-id]');
    if (!item) return;
    selectedServiceInput.value = item.getAttribute('data-id');
    Array.prototype.forEach.call(serviceList.querySelectorAll('li'), function (el) {
      el.classList.remove('selected');
    });
    item.classList.add('selected');
    loadSlots();
  });

  dateInput.addEventListener('change', loadSlots);

  slotList.addEventListener('click', function (event) {
    var item = event.target.closest('li[data-available="true"]');
    if (!item) return;
    selectedSlotInput.value = item.getAttribute('data-scheduled-at');
    Array.prototype.forEach.call(slotList.querySelectorAll('li'), function (el) {
      el.classList.remove('selected');
    });
    item.classList.add('selected');
    confirmButton.disabled = false;
  });
});
