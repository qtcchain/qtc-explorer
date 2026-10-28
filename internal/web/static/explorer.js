// Mobile navigation toggle and the current-section marker in the header. No dependencies.
(function () {
  var menu = document.querySelector('header .menu'), nav = document.querySelector('header nav');
  if (menu && nav) {
    menu.addEventListener('click', function () {
      var open = nav.classList.toggle('open');
      menu.setAttribute('aria-expanded', open ? 'true' : 'false');
      menu.textContent = open ? 'CLOSE ×' : 'MENU +';
    });
  }
  var path = location.pathname;
  Array.prototype.forEach.call(document.querySelectorAll('header nav a[href^="/"]'), function (a) {
    var href = a.getAttribute('href');
    if (href === '/') return;
    if (path === href || path.indexOf(href + '/') === 0 || (href === '/blocks' && path.indexOf('/block/') === 0)) {
      a.setAttribute('aria-current', 'page');
    }
  });
})();
