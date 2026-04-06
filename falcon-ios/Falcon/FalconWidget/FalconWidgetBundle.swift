//
//  FalconWidgetBundle.swift
//  FalconWidget
//
//  Created by Helmer Barcos on 06.04.26.
//

import WidgetKit
import SwiftUI

@main
struct FalconWidgetBundle: WidgetBundle {
    var body: some Widget {
        FalconWidget()
        FalconWidgetControl()
        FalconWidgetLiveActivity()
    }
}
