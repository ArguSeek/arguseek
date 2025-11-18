import React, { useState } from 'react';
import { Search, Zap, BookOpen, Network } from 'lucide-react';

export default function ArguSeekLogoShowcase() {
  const [theme, setTheme] = useState('light');
  const [activeTab, setActiveTab] = useState('full');
  
  const bgColor = theme === 'dark' ? 'bg-gray-900' : 'bg-white';
  const textColor = theme === 'dark' ? 'text-white' : 'text-gray-900';
  const mutedColor = theme === 'dark' ? 'text-gray-400' : 'text-gray-600';
  const borderColor = theme === 'dark' ? 'border-gray-700' : 'border-gray-200';
  
  // Color palette
  const primary = '#10B981'; // Emerald green - growth, knowledge, open source
  const accent = '#14B8A6'; // Teal - innovation, discovery, analysis
  
  // Logo component - Symbol only
  const LogoSymbol = ({ size = 48, showGradient = true }) => (
    <svg width={size} height={size} viewBox="0 0 100 100" fill="none">
      {/* Background circle */}
      <circle cx="50" cy="50" r="45" fill={showGradient ? "url(#gradient)" : primary} opacity="0.1"/>
      
      {/* Magnifying glass handle */}
      <path 
        d="M 65 65 L 80 80" 
        stroke={showGradient ? "url(#gradient)" : primary}
        strokeWidth="6" 
        strokeLinecap="round"
      />
      
      {/* Main circle of magnifying glass */}
      <circle 
        cx="45" 
        cy="45" 
        r="25" 
        stroke={showGradient ? "url(#gradient)" : primary}
        strokeWidth="6" 
        fill="none"
      />
      
      {/* Three dots inside representing multiple sources */}
      <circle cx="35" cy="40" r="3" fill={accent}/>
      <circle cx="45" cy="45" r="3" fill={accent}/>
      <circle cx="55" cy="50" r="3" fill={accent}/>
      
      {/* Gradient definition */}
      {showGradient && (
        <defs>
          <linearGradient id="gradient" x1="0%" y1="0%" x2="100%" y2="100%">
            <stop offset="0%" stopColor={primary} />
            <stop offset="100%" stopColor={accent} />
          </linearGradient>
        </defs>
      )}
    </svg>
  );
  
  // Alternative logo concept - Network/Connection theme
  const LogoSymbolAlt = ({ size = 48 }) => (
    <svg width={size} height={size} viewBox="0 0 100 100" fill="none">
      <defs>
        <linearGradient id="gradient2" x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor={primary} />
          <stop offset="100%" stopColor={accent} />
        </linearGradient>
      </defs>
      
      {/* Central node */}
      <circle cx="50" cy="50" r="12" fill="url(#gradient2)"/>
      
      {/* Orbiting nodes representing multiple sources */}
      <circle cx="30" cy="30" r="8" fill={primary} opacity="0.7"/>
      <circle cx="70" cy="30" r="8" fill={primary} opacity="0.7"/>
      <circle cx="30" cy="70" r="8" fill={accent} opacity="0.7"/>
      <circle cx="70" cy="70" r="8" fill={accent} opacity="0.7"/>
      
      {/* Connection lines */}
      <line x1="50" y1="50" x2="30" y2="30" stroke={primary} strokeWidth="2" opacity="0.4"/>
      <line x1="50" y1="50" x2="70" y2="30" stroke={primary} strokeWidth="2" opacity="0.4"/>
      <line x1="50" y1="50" x2="30" y2="70" stroke={accent} strokeWidth="2" opacity="0.4"/>
      <line x1="50" y1="50" x2="70" y2="70" stroke={accent} strokeWidth="2" opacity="0.4"/>
    </svg>
  );
  
  return (
    <div className={`min-h-screen ${bgColor} ${textColor} p-8 transition-colors`}>
      <div className="max-w-6xl mx-auto">
        
        {/* Header */}
        <div className="flex justify-between items-center mb-12">
          <div>
            <h1 className="text-4xl font-bold mb-2">ArguSeek Logo System</h1>
            <p className={mutedColor}>MCP Server for Deep Research & Knowledge Grounding</p>
          </div>
          <button
            onClick={() => setTheme(theme === 'dark' ? 'light' : 'dark')}
            className={`px-4 py-2 rounded-lg border ${borderColor} hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors`}
          >
            {theme === 'dark' ? '☀️ Light' : '🌙 Dark'}
          </button>
        </div>

        {/* Tab Navigation */}
        <div className={`flex gap-2 mb-8 border-b ${borderColor}`}>
          {['full', 'symbol', 'favicon', 'usage'].map(tab => (
            <button
              key={tab}
              onClick={() => setActiveTab(tab)}
              className={`px-4 py-2 border-b-2 transition-colors ${
                activeTab === tab 
                  ? 'border-blue-500 text-blue-500' 
                  : 'border-transparent ' + mutedColor
              }`}
            >
              {tab.charAt(0).toUpperCase() + tab.slice(1)}
            </button>
          ))}
        </div>

        {/* Full Logo Wordmark */}
        {activeTab === 'full' && (
          <div className="space-y-12">
            <section>
              <h2 className="text-2xl font-bold mb-6">Primary Wordmark</h2>
              <div className={`p-12 rounded-xl border ${borderColor} flex items-center justify-center`}>
                <div className="flex items-center gap-4">
                  <LogoSymbol size={64} />
                  <div>
                    <h1 className="text-5xl font-bold">
                      <span style={{ color: primary }}>Argu</span>
                      <span style={{ color: accent }}>Seek</span>
                    </h1>
                    <p className={`text-sm ${mutedColor} mt-1`}>Deep Research MCP Server</p>
                  </div>
                </div>
              </div>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-6">Horizontal Lockup</h2>
              <div className={`p-12 rounded-xl border ${borderColor} flex items-center justify-center`}>
                <div className="flex items-center gap-3">
                  <LogoSymbol size={48} />
                  <div className="text-4xl font-bold">
                    <span style={{ color: primary }}>Argu</span>
                    <span style={{ color: accent }}>Seek</span>
                  </div>
                </div>
              </div>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-6">Minimal Version</h2>
              <div className={`p-12 rounded-xl border ${borderColor} flex items-center justify-center`}>
                <div className="flex items-center gap-3">
                  <LogoSymbol size={32} showGradient={false} />
                  <div className="text-2xl font-semibold">
                    <span style={{ color: primary }}>Argu</span>
                    <span style={{ color: accent }}>Seek</span>
                  </div>
                </div>
              </div>
            </section>
          </div>
        )}

        {/* Symbol Only */}
        {activeTab === 'symbol' && (
          <div className="space-y-12">
            <section>
              <h2 className="text-2xl font-bold mb-6">Concept 1: Search & Discovery</h2>
              <p className={`${mutedColor} mb-6`}>Magnifying glass with three dots representing multiple sources</p>
              <div className="grid grid-cols-3 gap-8">
                <div className={`p-12 rounded-xl border ${borderColor} flex flex-col items-center justify-center`}>
                  <LogoSymbol size={96} />
                  <p className={`text-sm ${mutedColor} mt-4`}>Large</p>
                </div>
                <div className={`p-12 rounded-xl border ${borderColor} flex flex-col items-center justify-center`}>
                  <LogoSymbol size={64} />
                  <p className={`text-sm ${mutedColor} mt-4`}>Medium</p>
                </div>
                <div className={`p-12 rounded-xl border ${borderColor} flex flex-col items-center justify-center`}>
                  <LogoSymbol size={32} />
                  <p className={`text-sm ${mutedColor} mt-4`}>Small</p>
                </div>
              </div>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-6">Concept 2: Network & Connections</h2>
              <p className={`${mutedColor} mb-6`}>Central hub connected to multiple knowledge sources</p>
              <div className="grid grid-cols-3 gap-8">
                <div className={`p-12 rounded-xl border ${borderColor} flex flex-col items-center justify-center`}>
                  <LogoSymbolAlt size={96} />
                  <p className={`text-sm ${mutedColor} mt-4`}>Large</p>
                </div>
                <div className={`p-12 rounded-xl border ${borderColor} flex flex-col items-center justify-center`}>
                  <LogoSymbolAlt size={64} />
                  <p className={`text-sm ${mutedColor} mt-4`}>Medium</p>
                </div>
                <div className={`p-12 rounded-xl border ${borderColor} flex flex-col items-center justify-center`}>
                  <LogoSymbolAlt size={32} />
                  <p className={`text-sm ${mutedColor} mt-4`}>Small</p>
                </div>
              </div>
            </section>
          </div>
        )}

        {/* Favicon Variations */}
        {activeTab === 'favicon' && (
          <div className="space-y-12">
            <section>
              <h2 className="text-2xl font-bold mb-6">Favicon Sizes</h2>
              <div className="grid grid-cols-4 gap-6">
                {[64, 48, 32, 16].map(size => (
                  <div key={size} className={`p-8 rounded-xl border ${borderColor} flex flex-col items-center justify-center`}>
                    <div style={{ 
                      width: size, 
                      height: size, 
                      display: 'flex', 
                      alignItems: 'center', 
                      justifyContent: 'center',
                      marginBottom: '1rem'
                    }}>
                      <LogoSymbol size={size} />
                    </div>
                    <p className={`text-sm ${mutedColor}`}>{size}x{size}px</p>
                  </div>
                ))}
              </div>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-6">Simplified Favicon (for 16x16)</h2>
              <p className={`${mutedColor} mb-6`}>At very small sizes, a simplified version works better</p>
              <div className={`p-12 rounded-xl border ${borderColor} flex items-center justify-center gap-8`}>
                <div className="text-center">
                  <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
                    <circle cx="6" cy="6" r="4" stroke={primary} strokeWidth="1.5" fill="none"/>
                    <line x1="9" y1="9" x2="13" y2="13" stroke={primary} strokeWidth="1.5" strokeLinecap="round"/>
                    <circle cx="6" cy="6" r="1" fill={accent}/>
                  </svg>
                  <p className={`text-xs ${mutedColor} mt-2`}>Actual size</p>
                </div>
                <div className="text-center">
                  <svg width="64" height="64" viewBox="0 0 16 16" fill="none">
                    <circle cx="6" cy="6" r="4" stroke={primary} strokeWidth="1.5" fill="none"/>
                    <line x1="9" y1="9" x2="13" y2="13" stroke={primary} strokeWidth="1.5" strokeLinecap="round"/>
                    <circle cx="6" cy="6" r="1" fill={accent}/>
                  </svg>
                  <p className={`text-xs ${mutedColor} mt-2`}>4x scaled</p>
                </div>
              </div>
            </section>
          </div>
        )}

        {/* Usage Guidelines */}
        {activeTab === 'usage' && (
          <div className="space-y-8">
            <section>
              <h2 className="text-2xl font-bold mb-4">Color Palette</h2>
              <div className="grid grid-cols-2 gap-6">
                <div className="space-y-4">
                  <div className={`p-6 rounded-xl border ${borderColor}`}>
                    <div className="w-full h-24 rounded-lg mb-4" style={{ backgroundColor: primary }}></div>
                    <p className="font-mono text-sm">{primary}</p>
                    <p className={`text-sm ${mutedColor}`}>Primary Green - Growth, Knowledge & Open Source</p>
                  </div>
                </div>
                <div className="space-y-4">
                  <div className={`p-6 rounded-xl border ${borderColor}`}>
                    <div className="w-full h-24 rounded-lg mb-4" style={{ backgroundColor: accent }}></div>
                    <p className="font-mono text-sm">{accent}</p>
                    <p className={`text-sm ${mutedColor}`}>Accent Teal - Innovation, Discovery & Analysis</p>
                  </div>
                </div>
              </div>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4">Typography</h2>
              <div className={`p-6 rounded-xl border ${borderColor} space-y-4`}>
                <div>
                  <p className={`text-sm ${mutedColor} mb-2`}>Wordmark Typography</p>
                  <p className="text-3xl font-bold">ArguSeek</p>
                  <p className={`text-sm ${mutedColor} mt-1`}>Bold, Sans-serif (Inter, SF Pro, or similar)</p>
                </div>
                <div>
                  <p className={`text-sm ${mutedColor} mb-2`}>Tagline Typography</p>
                  <p className="text-base">Deep Research MCP Server</p>
                  <p className={`text-sm ${mutedColor} mt-1`}>Regular, Sans-serif</p>
                </div>
              </div>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4">Spacing & Clear Space</h2>
              <div className={`p-12 rounded-xl border ${borderColor} flex items-center justify-center relative`}>
                <div className="relative inline-flex items-center gap-3">
                  {/* Clear space indicators */}
                  <div className="absolute inset-0 border-2 border-dashed opacity-30" style={{ 
                    borderColor: primary,
                    margin: '-32px'
                  }}></div>
                  <LogoSymbol size={48} />
                  <div className="text-3xl font-bold">
                    <span style={{ color: primary }}>Argu</span>
                    <span style={{ color: accent }}>Seek</span>
                  </div>
                </div>
              </div>
              <p className={`text-sm ${mutedColor} mt-4 text-center`}>
                Maintain minimum clear space equal to the height of the icon around the logo
              </p>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4">Do's and Don'ts</h2>
              <div className="grid grid-cols-2 gap-6">
                <div className={`p-6 rounded-xl border-2 border-green-500 border-opacity-30`}>
                  <p className="text-green-600 dark:text-green-400 font-semibold mb-4">✓ Do</p>
                  <ul className={`space-y-2 text-sm ${mutedColor}`}>
                    <li>• Use approved color combinations</li>
                    <li>• Maintain proper spacing</li>
                    <li>• Use on contrasting backgrounds</li>
                    <li>• Scale proportionally</li>
                  </ul>
                </div>
                <div className={`p-6 rounded-xl border-2 border-red-500 border-opacity-30`}>
                  <p className="text-red-600 dark:text-red-400 font-semibold mb-4">✗ Don't</p>
                  <ul className={`space-y-2 text-sm ${mutedColor}`}>
                    <li>• Change the colors arbitrarily</li>
                    <li>• Rotate or distort the logo</li>
                    <li>• Add effects or shadows</li>
                    <li>• Place on busy backgrounds</li>
                  </ul>
                </div>
              </div>
            </section>
          </div>
        )}

        {/* Footer */}
        <div className={`mt-16 pt-8 border-t ${borderColor} text-center ${mutedColor}`}>
          <p className="text-sm">ArguSeek Logo System • Open Source MIT License</p>
          <p className="text-xs mt-2">Designed for dark & light themes, web, mobile, and developer tools</p>
        </div>
      </div>
    </div>
  );
}
