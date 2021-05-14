var app = angular.module('catsvsdogs', []);
var socket = io.connect({transports:['polling']});

var bg1 = document.getElementById('background-stats-1');
var bg2 = document.getElementById('background-stats-2');
var bg3 = document.getElementById('background-stats-3');

app.controller('statsCtrl', function($scope){
  $scope.aPercent = 33.33;
  $scope.bPercent = 33.33;
  $scope.cPercent = 33.34;

  var updateScores = function(){
    socket.on('scores', function (json) {
       data = JSON.parse(json);
       var a = parseInt(data.a || 0);
       var b = parseInt(data.b || 0);
       var c = parseInt(data.c || 0);

       var percentages = getPercentages(a, b, c);

       bg1.style.width = percentages.a + "%";
       bg2.style.width = percentages.b + "%";
       bg3.style.width = percentages.c + "%";

       $scope.$apply(function () {
         $scope.aPercent = percentages.a;
         $scope.bPercent = percentages.b;
         $scope.cPercent = percentages.c;
         $scope.total = a + b + c;
       });
    });
  };

  var init = function(){
    document.body.style.opacity=1;
    updateScores();
  };
  socket.on('message',function(data){
    init();
  });
});

function getPercentages(a, b, c) {
  var result = {};

  if (a + b + c > 0) {
    result.a = Math.round(a / (a + b + c) * 100);
    result.b = Math.round(b / (a + b + c) * 100);
    result.c = 100 - result.a - result.b;
  } else {
    result.a = result.b = 33.33;
    result.c = 33.34;
  }

  return result;
}