export const fallbackUsername = 'Guest';

const funnyWords = [
  'Бодрый Кабачок',
  'Сонный Пельмень',
  'Хитрый Вареник',
  'Ламповый Сырник',
  'Космический Бублик',
  'Шустрый Компот',
  'Тихий Самовар',
  'Веселый Укроп',
  'Серьезный Батон',
  'Мятный Блинчик',
  'Пушистый Квас',
  'Грозный Сухарик',
  'Нежный Чебурек',
  'Важный Пончик',
  'Секретный Огурчик',
  'Сахарный Кексик',
  'Пиксельный Пряник',
  'Турбо Ряженка',
  'Уютный Лапоть',
  'Блестящий Крендель'
];

export function randomUsername() {
  const word = funnyWords[Math.floor(Math.random() * funnyWords.length)] ?? fallbackUsername;
  const suffix = Math.floor(Math.random() * 1000);
  return `${word} ${suffix}`;
}
