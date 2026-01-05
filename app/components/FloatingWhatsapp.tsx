import { WHATSAPP_NUMBER } from '../constants';

export default function FloatingWhatsApp() {
  // Matches the styling from your original index.html
  return (
    <a 
      href={`https://wa.me/${WHATSAPP_NUMBER}?text=Hello,%20I%20need%20more%20info`} 
      target="_blank" 
      rel="noopener noreferrer"
      className="fixed bottom-6 right-6 bg-green-500 hover:bg-green-600 text-white p-4 rounded-full shadow-2xl transition-all transform hover:scale-110 z-50 flex items-center justify-center w-16 h-16"
      aria-label="Chat on WhatsApp"
    >
      <i className="fa-brands fa-whatsapp text-3xl"></i>
    </a>
  );
}