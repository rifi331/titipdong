// TitipDong — shared photo picker helper.
// Toggles the `capture` attribute on a hidden <input type="file"> before clicking,
// so the same input works for both "camera" and "gallery" flows on Android + iOS.
//
// Usage in templates:
//   <button onclick="pickPhoto('photo-input', true)">📷 Foto</button>
//   <button onclick="pickPhoto('photo-input', false)">🖼️ Galeri</button>
//   <input type="file" id="photo-input" name="photo" accept="image/*" class="hidden">
//
// `capture` being set before .click() is the trick: with it, the OS opens the camera;
// without it, the OS opens the file/gallery picker. This avoids the Xiaomi/Android
// behavior where tapping "Choose File" always opens the camera.
window.pickPhoto = function (inputId, withCamera) {
  var input = document.getElementById(inputId);
  if (!input) return;
  if (withCamera) {
    input.setAttribute('capture', 'environment');
  } else {
    input.removeAttribute('capture');
  }
  input.click();
};

// Show a small "✓ filename" hint when a file is picked (optional enhancement).
document.addEventListener('change', function (e) {
  if (e.target && e.target.type === 'file' && e.target.files && e.target.files[0]) {
    var box = document.getElementById('file-name');
    if (box) {
      box.classList.remove('hidden');
      var text = document.getElementById('file-name-text');
      if (text) {
        var name = e.target.files[0].name;
        text.textContent = name.length > 30 ? name.slice(0, 27) + '...' : name;
      }
    }
  }
}, true);
